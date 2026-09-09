package param

import "testing"

func TestValidateValue(t *testing.T) {
	ok := []struct{ typ, val string }{
		{"string", ""}, {"string", "anything 中文"},
		{"int", "0"}, {"int", "-42"}, {"int", "9007199254740993"},
		{"float", "3.14"}, {"float", "1e3"}, {"float", "7"},
		{"bool", "true"}, {"bool", "false"},
		{"json", `{"a":1}`}, {"json", `[1,2]`}, {"json", `"s"`}, {"json", "null"},
	}
	for _, c := range ok {
		if err := ValidateValue(c.typ, c.val); err != nil {
			t.Errorf("%s %q should pass: %v", c.typ, c.val, err)
		}
	}
	bad := []struct{ typ, val string }{
		{"int", ""}, {"int", "1.5"}, {"int", "abc"}, {"int", " 1"},
		{"float", "x"}, {"float", ""},
		{"bool", "yes"}, {"bool", "1"}, {"bool", "True"}, {"bool", ""},
		{"json", "{a:1}"}, {"json", ""}, {"json", "{"},
		{"date", "2024-01-01"},
	}
	for _, c := range bad {
		if err := ValidateValue(c.typ, c.val); err == nil {
			t.Errorf("%s %q should be rejected", c.typ, c.val)
		}
	}
}

func TestDefaultsAreSelfConsistent(t *testing.T) {
	for _, d := range Defaults {
		if err := ValidateValue(d.Type, d.Value); err != nil {
			t.Errorf("default %s=%q (%s): %v", d.Key, d.Value, d.Type, err)
		}
	}
}
