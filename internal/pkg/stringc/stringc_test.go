package stringc

import "testing"

func TestSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello_world"},
		{"helloWorld", "hello_world"},
		{"HelloWorld", "hello_world"},
		{"hello-world", "hello_world"},
		{"hello_world", "hello_world"},
		{"HTTPRequest", "http_request"},
		{"SimpleXMLParser", "simple_xml_parser"},
		{"  lots   of spaces  ", "lots_of_spaces"},
	}

	for _, test := range tests {
		result := SnakeCase(test.input)
		if result != test.expected {
			t.Errorf("SnakeCase(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestTrimAndCase(t *testing.T) {
	if got := TrimAndLowerCase("  Hello World \n"); got != "hello world" {
		t.Errorf("TrimAndLowerCase = %q; want %q", got, "hello world")
	}
	if got := TrimAndUpperCase("  Hello World \n"); got != "HELLO WORLD" {
		t.Errorf("TrimAndUpperCase = %q; want %q", got, "HELLO WORLD")
	}
}
