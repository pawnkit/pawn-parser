package parser

import "testing"

func FuzzParse(f *testing.F) {
	seeds := []string{
		"public OnGameModeInit() { return 1; }",
		"#define FOO(%0,%1) %0 + %1",
		"enum E { A, B, C }",
		"#if defined X\nnew x;\n#else\nnew y;\n#endif\n",
		"stock F() {\n#if defined A\n if (x) {\n#else\n if (y) {\n#endif\n r(); } }",
		"new Float:x[10] = {1.0, 2.0};",
		"switch (x) { case 1, 2: break; default: break; }",
		"forward Foo(playerid, const name[], Float:x = 0.0, ...);",
		"{{{{{",
		"public Foo(",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on input %q: %v", src, r)
			}
		}()
		Parse([]byte(src))
	})
}
