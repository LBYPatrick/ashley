package skills

import (
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"strings"
)

// Ashley keeps compatibility overrides local; gonja's shared environment is never mutated.
var templateEnvironment = compatibleEnvironment()

func compatibleEnvironment() *exec.Environment {
	environment := *gonja.DefaultEnvironment
	strMethods := map[string]exec.Method[string]{}
	for _, name := range strings.Fields("capitalize capwords casefold center count encode endswith expandtabs find format format_map isalnum isalpha isascii isdecimal isdigit islower isnumeric isprintable isspace istitle isupper join ljust lower lstrip partition removeprefix removesuffix replace rfind rjust rpartition rsplit rstrip split splitlines startswith strip swapcase title upper zfill") {
		if method, ok := environment.Methods.Str.Get(name); ok {
			strMethods[name] = method
		}
	}
	strMethods["replace"] = func(self string, _ *exec.Value, args *exec.VarArgs) (any, error) {
		var old, replacement string
		var count int
		if err := args.Take(exec.PositionalArgument("old", nil, exec.StringArgument(&old)), exec.PositionalArgument("new", nil, exec.StringArgument(&replacement)), exec.PositionalArgument("count", exec.AsValue(-1), exec.IntArgument(&count))); err != nil {
			return nil, exec.ErrInvalidCall(err)
		}
		return strings.Replace(self, old, replacement, count), nil
	}
	environment.Methods.Str = exec.NewMethodSet(strMethods)
	dictMethods := map[string]exec.Method[map[string]any]{}
	for _, name := range strings.Fields("keys values items get pop setdefault update copy clear") {
		if method, ok := environment.Methods.Dict.Get(name); ok {
			dictMethods[name] = method
		}
	}
	for _, name := range []string{"items", "keys", "values"} {
		original := dictMethods[name]
		dictMethods[name] = func(self map[string]any, value *exec.Value, args *exec.VarArgs) (any, error) {
			pairs, ok := dictionaryPairs(value)
			if !ok {
				return original(self, value, args)
			}
			if err := args.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			result := make([]any, 0, len(pairs))
			for _, pair := range pairs {
				switch name {
				case "items":
					result = append(result, []any{pair.Key.Interface(), pair.Value.Interface()})
				case "keys":
					result = append(result, pair.Key.Interface())
				case "values":
					result = append(result, pair.Value.Interface())
				}
			}
			return result, nil
		}
	}
	environment.Methods.Dict = exec.NewMethodSet(dictMethods)
	environment.Filters = exec.NewFilterSet(map[string]exec.FilterFunction{}).Update(environment.Filters)
	original, _ := environment.Filters.Get("dictsort")
	environment.Filters.Replace("dictsort", func(e *exec.Evaluator, value *exec.Value, args *exec.VarArgs) *exec.Value {
		if pairs, ok := dictionaryPairs(value); ok {
			mapped := map[any]any{}
			for _, pair := range pairs {
				mapped[pair.Key.Interface()] = pair.Value.Interface()
			}
			value = exec.AsValue(mapped)
		}
		return original(e, value, args)
	})
	return &environment
}
func dictionaryPairs(value *exec.Value) ([]*exec.Pair, bool) {
	switch dict := value.Interface().(type) {
	case exec.Dict:
		return dict.Pairs, true
	case *exec.Dict:
		return dict.Pairs, true
	}
	return nil, false
}
