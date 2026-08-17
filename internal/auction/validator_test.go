package auction

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`coach addmany 3 John Jane Bob`, []string{"coach", "addmany", "3", "John", "Jane", "Bob"}},
		{`session new "Serie A"`, []string{"session", "new", "Serie A"}},
		{`coach edit teamname Bob ""`, []string{"coach", "edit", "teamname", "Bob", ""}},
		{`"" "x"`, []string{"", "x"}},
		{`a"b c"`, []string{"ab c"}},
		{`  spaced  out  `, []string{"spaced", "out"}},
	}
	for _, c := range cases {
		if got := SplitArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitArgs(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestJoinPlayerFlag(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"--player", "Lionel", "Messi", "--role", "fw"}, []string{"--player", "Lionel Messi", "--role", "fw"}},
		{[]string{"-p", "Lionel", "Messi"}, []string{"-p", "Lionel Messi"}},
		{[]string{"--player", "Lionel Messi"}, []string{"--player", "Lionel Messi"}},
		{[]string{"--player", "Messi", "--cost", "120"}, []string{"--player", "Messi", "--cost", "120"}},
		{[]string{"--role", "fw"}, []string{"--role", "fw"}},
		{[]string{"--player"}, []string{"--player"}},
	}
	for _, c := range cases {
		if got := JoinPlayerFlag(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("JoinPlayerFlag(%#v) = %#v, want %#v", c.in, got, c.want)
		}
	}
}
