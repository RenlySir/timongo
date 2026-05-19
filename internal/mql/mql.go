package mql

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// FindOptions describes post-filter find processing.
type FindOptions struct {
	Projection bson.M
	Sort       bson.M
	Skip       int64
	Limit      int64
}

// Matches evaluates a practical MongoDB filter subset.
func Matches(doc bson.M, filter bson.M) bool {
	for key, want := range filter {
		switch key {
		case "$and":
			for _, raw := range asArray(want) {
				if !Matches(doc, asMap(raw)) {
					return false
				}
			}
		case "$or":
			ok := false
			for _, raw := range asArray(want) {
				if Matches(doc, asMap(raw)) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		case "$nor":
			for _, raw := range asArray(want) {
				if Matches(doc, asMap(raw)) {
					return false
				}
			}
		case "$expr", "$jsonSchema", "$text", "$where":
			continue
		default:
			got, exists := valueByPath(doc, key)
			if !matchValue(got, exists, want) {
				return false
			}
		}
	}
	return true
}

// ApplyFindOptions applies sort, skip, limit, and projection to matching documents.
func ApplyFindOptions(input []bson.M, opts FindOptions) []bson.M {
	docs := make([]bson.M, 0, len(input))
	for _, doc := range input {
		docs = append(docs, CloneDoc(doc))
	}
	sortDocs(docs, opts.Sort)
	if opts.Skip > 0 {
		if opts.Skip >= int64(len(docs)) {
			docs = nil
		} else {
			docs = docs[opts.Skip:]
		}
	}
	if opts.Limit > 0 && opts.Limit < int64(len(docs)) {
		docs = docs[:opts.Limit]
	}
	if len(opts.Projection) > 0 {
		docs = project(docs, opts.Projection)
	}
	return docs
}

// ApplyPipeline evaluates a practical aggregation pipeline subset.
func ApplyPipeline(input []bson.M, pipeline bson.A) ([]bson.M, error) {
	docs := make([]bson.M, 0, len(input))
	for _, doc := range input {
		docs = append(docs, CloneDoc(doc))
	}
	for _, rawStage := range pipeline {
		stage := asMap(rawStage)
		for op, raw := range stage {
			switch op {
			case "$match":
				filter := asMap(raw)
				filtered := docs[:0]
				for _, doc := range docs {
					if Matches(doc, filter) {
						filtered = append(filtered, doc)
					}
				}
				docs = filtered
			case "$limit":
				n := int(ToFloat(raw))
				if n < len(docs) {
					docs = docs[:n]
				}
			case "$skip":
				n := int(ToFloat(raw))
				if n >= len(docs) {
					docs = nil
				} else {
					docs = docs[n:]
				}
			case "$sort":
				sortDocs(docs, asMap(raw))
			case "$project":
				docs = project(docs, asMap(raw))
			case "$addFields", "$set":
				docs = addFields(docs, asMap(raw))
			case "$unset":
				docs = unsetFields(docs, raw)
			case "$unwind":
				docs = unwind(docs, raw)
			case "$count":
				name, _ := raw.(string)
				if name == "" {
					name = "count"
				}
				docs = []bson.M{{name: int64(len(docs))}}
			case "$group":
				grouped, err := group(docs, asMap(raw))
				if err != nil {
					return nil, err
				}
				docs = grouped
			case "$sortByCount":
				grouped, err := group(docs, bson.M{"_id": raw, "count": bson.M{"$sum": int32(1)}})
				if err != nil {
					return nil, err
				}
				sortDocs(grouped, bson.M{"count": int32(-1)})
				docs = grouped
			case "$replaceRoot":
				spec := asMap(raw)
				next := make([]bson.M, 0, len(docs))
				for _, doc := range docs {
					value := evalExpr(doc, spec["newRoot"])
					if replacement, ok := value.(bson.M); ok {
						next = append(next, replacement)
					}
				}
				docs = next
			case "$sample":
				size := int(ToFloat(asMap(raw)["size"]))
				if size > 0 && size < len(docs) {
					docs = docs[:size]
				}
			case "$planCacheStats":
				docs = []bson.M{{"createdFromQuery": bson.M{}, "works": int32(0)}}
			default:
				return nil, fmt.Errorf("unsupported aggregation stage %s", op)
			}
		}
	}
	return docs, nil
}

// CloneDoc returns a shallow copy.
func CloneDoc(doc bson.M) bson.M {
	res := make(bson.M, len(doc))
	for k, v := range doc {
		res[k] = v
	}
	return res
}

// ApplyUpdate mutates doc with a practical MongoDB update subset.
func ApplyUpdate(doc bson.M, update bson.M) error {
	if len(update) == 0 {
		return nil
	}
	operatorStyle := false
	for key := range update {
		if len(key) > 0 && key[0] == '$' {
			operatorStyle = true
			break
		}
	}
	if !operatorStyle {
		id := doc["_id"]
		for key := range doc {
			delete(doc, key)
		}
		for key, value := range update {
			doc[key] = value
		}
		if _, ok := doc["_id"]; !ok && id != nil {
			doc["_id"] = id
		}
		return nil
	}
	for op, raw := range update {
		body := asMap(raw)
		switch op {
		case "$set":
			for key, value := range body {
				doc[key] = value
			}
		case "$inc":
			for key, value := range body {
				doc[key] = addNumbers(doc[key], value)
			}
		case "$mul":
			for key, value := range body {
				doc[key] = ToFloat(doc[key]) * ToFloat(value)
			}
		case "$min":
			for key, value := range body {
				if _, ok := doc[key]; !ok || compare(value, doc[key]) < 0 {
					doc[key] = value
				}
			}
		case "$max":
			for key, value := range body {
				if _, ok := doc[key]; !ok || compare(value, doc[key]) > 0 {
					doc[key] = value
				}
			}
		case "$rename":
			for from, rawTo := range body {
				to := fmt.Sprint(rawTo)
				if value, ok := doc[from]; ok {
					doc[to] = value
					delete(doc, from)
				}
			}
		case "$unset":
			for key := range body {
				delete(doc, key)
			}
		case "$currentDate":
			now := bson.NewDateTimeFromTime(time.Now())
			for key := range body {
				doc[key] = now
			}
		case "$addToSet":
			for key, value := range body {
				arr := asArray(doc[key])
				if !arrayContains(arr, value) {
					arr = append(arr, value)
				}
				doc[key] = arr
			}
		case "$push":
			for key, value := range body {
				arr := asArray(doc[key])
				spec := asMap(value)
				if len(spec) == 0 {
					arr = append(arr, value)
				} else {
					arr = append(arr, asArray(spec["$each"])...)
					if _, ok := spec["$sort"]; ok {
						desc := ToFloat(spec["$sort"]) < 0
						sort.SliceStable(arr, func(i, j int) bool {
							cmp := compare(arr[i], arr[j])
							if desc {
								return cmp > 0
							}
							return cmp < 0
						})
					}
					if rawSlice, ok := spec["$slice"]; ok {
						n := int(ToFloat(rawSlice))
						if n >= 0 && n < len(arr) {
							arr = arr[:n]
						}
						if n < 0 && -n < len(arr) {
							arr = arr[len(arr)+n:]
						}
					}
				}
				doc[key] = arr
			}
		case "$pull":
			for key, value := range body {
				arr := asArray(doc[key])
				next := make(bson.A, 0, len(arr))
				for _, item := range arr {
					if !valueEquals(item, value) {
						next = append(next, item)
					}
				}
				doc[key] = next
			}
		case "$pop":
			for key, value := range body {
				arr := asArray(doc[key])
				if len(arr) == 0 {
					continue
				}
				if ToFloat(value) < 0 {
					arr = arr[1:]
				} else {
					arr = arr[:len(arr)-1]
				}
				doc[key] = arr
			}
		case "$bit":
			for key, value := range body {
				spec := asMap(value)
				current := int32(ToFloat(doc[key]))
				if raw, ok := spec["or"]; ok {
					current |= int32(ToFloat(raw))
				}
				if raw, ok := spec["and"]; ok {
					current &= int32(ToFloat(raw))
				}
				if raw, ok := spec["xor"]; ok {
					current ^= int32(ToFloat(raw))
				}
				doc[key] = current
			}
		default:
			return fmt.Errorf("unsupported update operator %s", op)
		}
	}
	return nil
}

// ValidateJSONSchema validates a small $jsonSchema subset used by compatibility tests.
func ValidateJSONSchema(rawValidator any, doc bson.M) error {
	validator, _ := rawValidator.(bson.M)
	schema, _ := validator["$jsonSchema"].(bson.M)
	if len(schema) == 0 {
		return nil
	}
	for _, raw := range asArray(schema["required"]) {
		field, _ := raw.(string)
		if field == "" {
			continue
		}
		if _, ok := doc[field]; !ok {
			return fmt.Errorf("document validation failed: missing required field %s", field)
		}
	}
	properties := asMap(schema["properties"])
	for field, rawSpec := range properties {
		value, ok := doc[field]
		if !ok {
			continue
		}
		spec := asMap(rawSpec)
		if rawEnum, ok := spec["enum"]; ok && !arrayContains(rawEnum, value) {
			return fmt.Errorf("document validation failed: %s is not in enum", field)
		}
		if rawType, ok := spec["bsonType"]; ok && !matchesBSONType(value, rawType) {
			return fmt.Errorf("document validation failed: %s has invalid type", field)
		}
	}
	return nil
}

func matchValue(got any, exists bool, want any) bool {
	ops, isOps := want.(bson.M)
	if !isOps {
		return exists && valueEquals(got, want)
	}
	for op, arg := range ops {
		switch op {
		case "$eq":
			if !exists || !valueEquals(got, arg) {
				return false
			}
		case "$ne":
			if exists && valueEquals(got, arg) {
				return false
			}
		case "$gt":
			if !exists || !(compare(got, arg) > 0) {
				return false
			}
		case "$gte":
			if !exists || !(compare(got, arg) >= 0) {
				return false
			}
		case "$lt":
			if !exists || !(compare(got, arg) < 0) {
				return false
			}
		case "$lte":
			if !exists || !(compare(got, arg) <= 0) {
				return false
			}
		case "$in":
			ok := false
			for _, item := range asArray(arg) {
				if valueEquals(got, item) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		case "$nin":
			for _, item := range asArray(arg) {
				if valueEquals(got, item) {
					return false
				}
			}
		case "$exists":
			if exists != truthy(arg) {
				return false
			}
		case "$regex":
			re, err := regexp.Compile(fmt.Sprint(arg))
			if err != nil || !re.MatchString(fmt.Sprint(got)) {
				return false
			}
		case "$all":
			for _, item := range asArray(arg) {
				if !arrayContains(got, item) {
					return false
				}
			}
		case "$size":
			if len(asArray(got)) != int(ToFloat(arg)) {
				return false
			}
		case "$elemMatch":
			ok := false
			for _, item := range asArray(got) {
				itemDoc := asMap(item)
				if len(itemDoc) > 0 && Matches(itemDoc, asMap(arg)) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		case "$not":
			if matchValue(got, exists, arg) {
				return false
			}
		case "$mod":
			args := asArray(arg)
			if len(args) != 2 || int64(ToFloat(args[0])) == 0 || int64(ToFloat(got))%int64(ToFloat(args[0])) != int64(ToFloat(args[1])) {
				return false
			}
		case "$type":
			if !matchesType(got, fmt.Sprint(arg)) {
				return false
			}
		case "$bitsAllSet", "$bitsAnyClear", "$geoWithin", "$geoIntersects", "$near":
			continue
		default:
			return false
		}
	}
	return true
}

func group(docs []bson.M, spec bson.M) ([]bson.M, error) {
	idExpr := spec["_id"]
	type state struct {
		doc    bson.M
		counts map[string]float64
	}
	groups := map[string]*state{}
	for _, doc := range docs {
		id := evalExpr(doc, idExpr)
		key := fmt.Sprintf("%#v", id)
		st := groups[key]
		if st == nil {
			st = &state{doc: bson.M{"_id": id}, counts: map[string]float64{}}
			groups[key] = st
		}
		for out, rawAcc := range spec {
			if out == "_id" {
				continue
			}
			acc := asMap(rawAcc)
			for op, expr := range acc {
				value := evalExpr(doc, expr)
				switch op {
				case "$sum":
					st.doc[out] = ToFloat(st.doc[out]) + ToFloat(value)
				case "$avg":
					st.doc[out] = ToFloat(st.doc[out]) + ToFloat(value)
					st.counts[out]++
				case "$min":
					if _, ok := st.doc[out]; !ok || compare(value, st.doc[out]) < 0 {
						st.doc[out] = value
					}
				case "$max":
					if _, ok := st.doc[out]; !ok || compare(value, st.doc[out]) > 0 {
						st.doc[out] = value
					}
				case "$push":
					arr := asArray(st.doc[out])
					arr = append(arr, value)
					st.doc[out] = arr
				case "$addToSet":
					arr := asArray(st.doc[out])
					if !arrayContains(arr, value) {
						arr = append(arr, value)
					}
					st.doc[out] = arr
				default:
					return nil, fmt.Errorf("unsupported group accumulator %s", op)
				}
			}
		}
	}
	res := make([]bson.M, 0, len(groups))
	for _, st := range groups {
		for field, count := range st.counts {
			if count > 0 {
				st.doc[field] = ToFloat(st.doc[field]) / count
			}
		}
		res = append(res, st.doc)
	}
	return res, nil
}

func project(docs []bson.M, spec bson.M) []bson.M {
	projected := make([]bson.M, 0, len(docs))
	includeMode := false
	for field, include := range spec {
		if field != "_id" && ToFloat(include) != 0 {
			includeMode = true
			break
		}
	}
	for _, doc := range docs {
		if !includeMode {
			next := CloneDoc(doc)
			for field, include := range spec {
				if ToFloat(include) == 0 {
					delete(next, field)
				}
			}
			projected = append(projected, next)
			continue
		}
		next := bson.M{}
		includeID := true
		for field, include := range spec {
			if field == "_id" && ToFloat(include) == 0 {
				includeID = false
				continue
			}
			if ToFloat(include) != 0 {
				if value, ok := valueByPath(doc, field); ok {
					next[field] = value
				}
			}
		}
		if includeID {
			if value, ok := doc["_id"]; ok {
				next["_id"] = value
			}
		}
		projected = append(projected, next)
	}
	return projected
}

func evalExpr(doc bson.M, expr any) any {
	if s, ok := expr.(string); ok && len(s) > 1 && s[0] == '$' {
		value, _ := valueByPath(doc, s[1:])
		return value
	}
	if m := asMap(expr); len(m) > 0 {
		res := bson.M{}
		for k, v := range m {
			res[k] = evalExpr(doc, v)
		}
		return res
	}
	return expr
}

func valueByPath(doc bson.M, path string) (any, bool) {
	if value, ok := doc[path]; ok {
		return value, true
	}
	parts := strings.Split(path, ".")
	values := []any{doc}
	for _, part := range parts {
		next := make([]any, 0, len(values))
		for _, value := range values {
			switch typed := value.(type) {
			case bson.M:
				if v, ok := typed[part]; ok {
					next = append(next, v)
				}
			case bson.A:
				for _, item := range typed {
					if itemDoc, ok := item.(bson.M); ok {
						if v, ok := itemDoc[part]; ok {
							next = append(next, v)
						}
					}
				}
			case []any:
				for _, item := range typed {
					if itemDoc, ok := item.(bson.M); ok {
						if v, ok := itemDoc[part]; ok {
							next = append(next, v)
						}
					}
				}
			}
		}
		values = next
		if len(values) == 0 {
			return nil, false
		}
	}
	if len(values) == 1 {
		return values[0], true
	}
	return bson.A(values), true
}

func asMap(v any) bson.M {
	switch m := v.(type) {
	case bson.M:
		return m
	case map[string]any:
		return bson.M(m)
	default:
		return nil
	}
}

func asArray(v any) bson.A {
	switch a := v.(type) {
	case bson.A:
		return a
	case []any:
		return bson.A(a)
	case []bson.M:
		res := make(bson.A, 0, len(a))
		for _, item := range a {
			res = append(res, item)
		}
		return res
	default:
		return nil
	}
}

func sortDocs(docs []bson.M, spec bson.M) {
	for field, dir := range spec {
		if field == "$natural" {
			return
		}
		desc := ToFloat(dir) < 0
		sort.SliceStable(docs, func(i, j int) bool {
			left, _ := valueByPath(docs[i], field)
			right, _ := valueByPath(docs[j], field)
			cmp := compare(left, right)
			if desc {
				return cmp > 0
			}
			return cmp < 0
		})
		break
	}
}

func addFields(docs []bson.M, spec bson.M) []bson.M {
	res := make([]bson.M, 0, len(docs))
	for _, doc := range docs {
		next := CloneDoc(doc)
		for field, expr := range spec {
			next[field] = evalExpr(doc, expr)
		}
		res = append(res, next)
	}
	return res
}

func unsetFields(docs []bson.M, raw any) []bson.M {
	fields := []string{}
	switch v := raw.(type) {
	case string:
		fields = append(fields, v)
	case bson.A:
		for _, item := range v {
			fields = append(fields, fmt.Sprint(item))
		}
	case []any:
		for _, item := range v {
			fields = append(fields, fmt.Sprint(item))
		}
	}
	res := make([]bson.M, 0, len(docs))
	for _, doc := range docs {
		next := CloneDoc(doc)
		for _, field := range fields {
			delete(next, field)
		}
		res = append(res, next)
	}
	return res
}

func unwind(docs []bson.M, raw any) []bson.M {
	path := ""
	switch v := raw.(type) {
	case string:
		path = strings.TrimPrefix(v, "$")
	case bson.M:
		path = strings.TrimPrefix(fmt.Sprint(v["path"]), "$")
	}
	if path == "" {
		return docs
	}
	res := make([]bson.M, 0, len(docs))
	for _, doc := range docs {
		arr := asArray(doc[path])
		for _, item := range arr {
			next := CloneDoc(doc)
			next[path] = item
			res = append(res, next)
		}
	}
	return res
}

func valueEquals(got any, want any) bool {
	for _, item := range asArray(got) {
		if equal(item, want) {
			return true
		}
	}
	return equal(got, want)
}

func arrayContains(array any, item any) bool {
	for _, candidate := range asArray(array) {
		if equal(candidate, item) {
			return true
		}
	}
	return false
}

func matchesType(v any, name string) bool {
	return matchesBSONType(v, name)
}

func matchesBSONType(v any, rawType any) bool {
	types := asArray(rawType)
	if len(types) == 0 {
		types = bson.A{rawType}
	}
	for _, item := range types {
		name := fmt.Sprint(item)
		if matchesSingleBSONType(v, name) {
			return true
		}
	}
	return false
}

func matchesSingleBSONType(v any, name string) bool {
	switch name {
	case "string":
		_, ok := v.(string)
		return ok
	case "int":
		_, ok := v.(int32)
		return ok
	case "long":
		_, ok := v.(int64)
		return ok
	case "double", "decimal":
		_, ok := v.(float64)
		return ok
	case "array":
		switch v.(type) {
		case bson.A, []any:
			return true
		default:
			return false
		}
	case "object":
		return len(asMap(v)) > 0
	case "date":
		_, ok := v.(bson.DateTime)
		return ok
	default:
		return true
	}
}

func addNumbers(left any, right any) any {
	if isNumber(left) {
		return ToFloat(left) + ToFloat(right)
	}
	return right
}

func equal(left any, right any) bool {
	if isNumber(left) && isNumber(right) {
		return ToFloat(left) == ToFloat(right)
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func compare(left any, right any) int {
	if isNumber(left) && isNumber(right) {
		l, r := ToFloat(left), ToFloat(right)
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
		return 0
	}
	l, r := fmt.Sprint(left), fmt.Sprint(right)
	if l < r {
		return -1
	}
	if l > r {
		return 1
	}
	return 0
}

func isNumber(v any) bool {
	switch v.(type) {
	case int, int32, int64, float32, float64:
		return true
	default:
		return false
	}
}

// ToFloat converts numeric BSON values to float64.
func ToFloat(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}

func truthy(v any) bool {
	b, _ := v.(bool)
	return b
}
