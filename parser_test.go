package parser

import "testing"

func mustNotBeBroken(t *testing.T, f *File, src string) {
	t.Helper()
	if f.Broken {
		t.Fatalf("parser reported Broken for input:\n%s", src)
	}
}

func TestParseBasicFunction(t *testing.T) {
	t.Parallel()
	src := "public OnGameModeInit()\n{\n    new value = 1;\n    return 1;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 {
		t.Fatalf("expected 1 top-level item, got %d", len(f.Root.Children))
	}
	fn := f.Root.Children[0]
	if fn.Kind != KindFunctionDefinition {
		t.Fatalf("expected function_definition, got %v", fn.Kind)
	}
	if fn.HasError {
		t.Fatalf("function unexpectedly HasError")
	}
}

func TestParseCaseLabelWithUpperCamelConstant(t *testing.T) {
	t.Parallel()
	src := "stock F(dialogid) {\n    switch (dialogid) {\n        case DIALOG_CHOOSE_MAP:\n        {\n            return 1;\n        }\n    }\n    return 0;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	fn := f.Root.Children[0]
	if fn.HasError {
		t.Fatalf("function unexpectedly HasError for upper-camel case label:\n%s", src)
	}
}

func TestParseNamedCallArgument(t *testing.T) {
	t.Parallel()
	src := "stock F(playerid) {\n    return ShowCreatorDialog(playerid, DIALOG_OPEN_MAP, .versatile = true);\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	fn := f.Root.Children[0]
	if fn.HasError {
		t.Fatalf("function unexpectedly HasError for named call argument:\n%s", src)
	}
}

func TestParseAdjacentStringConcat(t *testing.T) {
	t.Parallel()
	src := "stock const X[] = \"a\" \"b\" \"c\";\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if f.Root.Children[0].HasError {
		t.Fatalf("unexpectedly HasError for adjacent string concat:\n%s", src)
	}
}

func TestParseStringizeOperator(t *testing.T) {
	t.Parallel()
	src := "stock const X[] = #VERSION_MAJOR \".\" #VERSION_MINOR;\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if f.Root.Children[0].HasError {
		t.Fatalf("unexpectedly HasError for '#IDENT' stringize operator:\n%s", src)
	}
}

func TestParseUnsignedShiftOperator(t *testing.T) {
	t.Parallel()
	src := "stock F(x) {\n    return x >>> 1;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	fn := f.Root.Children[0]
	if fn.HasError {
		t.Fatalf("function unexpectedly HasError for '>>>' operator:\n%s", src)
	}
	block := fn.Field("body")
	ret := block.Children[0]
	if ret.Kind != KindReturnStatement {
		t.Fatalf("expected return_statement, got %v", ret.Kind)
	}
	expr := ret.Field("value")
	if expr.Kind != KindBinaryExpression {
		t.Fatalf("expected '>>>' to parse as a single binary_expression, got %v", expr.Kind)
	}
	if expr.HasError {
		t.Fatalf("binary_expression unexpectedly HasError")
	}
}

func TestParseDirectives(t *testing.T) {
	t.Parallel()
	src := "#include <a_samp>\n#define FOO 1\n#define BAR(%0,%1) SendClientMessage(%0, -1, %1)\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 3 {
		t.Fatalf("expected 3 items, got %d: %+v", len(f.Root.Children), kindsOf(f.Root.Children))
	}
	if f.Root.Children[0].Kind != KindDirectiveInclude {
		t.Fatalf("expected include, got %v", f.Root.Children[0].Kind)
	}
	define := f.Root.Children[2]
	if define.Kind != KindDirectiveDefine {
		t.Fatalf("expected define, got %v", define.Kind)
	}
	if define.Field("parameters") == nil {
		t.Fatalf("expected macro parameter list")
	}
	value := define.Field("value")
	if value == nil || value.HasError {
		t.Fatalf("expected macro body to parse cleanly as an expression, got %+v", value)
	}
	if value.Kind != KindCallExpression {
		t.Fatalf("expected call expression body, got %v", value.Kind)
	}
}

func TestParseConditionalRegionClean(t *testing.T) {
	t.Parallel()
	src := "#if defined DEBUG\nforward DebugLog(const message[]);\nnew gDebugLevel = 2;\n#else\nnative WriteLog(const message[]);\n#endif\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 {
		t.Fatalf("expected 1 top-level item (the conditional region), got %d: %v", len(f.Root.Children), kindsOf(f.Root.Children))
	}
	region := f.Root.Children[0]
	if region.Kind != KindConditionalRegion {
		t.Fatalf("expected conditional_region, got %v", region.Kind)
	}
	if len(region.Children) != 3 {
		t.Fatalf("expected 3 branches (if/else/endif), got %d", len(region.Children))
	}
	ifBranch := region.Children[0]
	if len(ifBranch.Children) != 3 {
		t.Fatalf("expected if-branch to have directive+2 items, got %d: %v", len(ifBranch.Children), kindsOf(ifBranch.Children))
	}
}

func TestParseEnum(t *testing.T) {
	t.Parallel()
	src := "enum E_PLAYER_DATA\n{\n    bool:pLoggedIn,\n    pScore,\n    Float:pHealth,\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	en := f.Root.Children[0]
	if en.Kind != KindEnumDeclaration {
		t.Fatalf("expected enum_declaration, got %v", en.Kind)
	}
	body := en.Field("body")
	if body == nil || len(body.Children) != 3 {
		t.Fatalf("expected 3 enum entries, got %+v", body)
	}
}

func TestParseMalformedDoesNotPanicOrBreak(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"public Foo(",
		"if (x",
		"enum { a, b",
		"#define",
		"#if defined X\nnew x;\n",
		"switch (x) { case",
		"new x = ",
		"{{{{{{{{",
		"public Foo() { if (a) { if (b) { } }",
	}
	for _, src := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", src, r)
				}
			}()
			f := Parse([]byte(src))
			_ = f
		}()
	}
}

func TestSharedBraceConditionalHasExplicitNode(t *testing.T) {
	t.Parallel()
	src := "stock F() {\n#if defined A\n    if (x) {\n#else\n    if (y) {\n#endif\n        return 1;\n    }\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	fn := f.Root.Children[0]
	if fn.Kind != KindFunctionDefinition {
		t.Fatalf("expected function_definition, got %v", fn.Kind)
	}
	body := fn.Field("body")
	if body == nil || len(body.Children) == 0 || body.Children[0].Kind != KindSharedConditional {
		t.Fatalf("expected shared_conditional in function body, got %+v", body)
	}
	if fn.HasError || body.HasError {
		t.Fatal("shared conditional must not poison its containing function")
	}
}

func TestConditionalFunctionHeadersShareBody(t *testing.T) {
	t.Parallel()
	src := "#if defined LONG\npublic F(value, extra)\n#else\npublic F(value)\n#endif\n{\n    return value;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 || f.Root.Children[0].Kind != KindConditionalFunction {
		t.Fatalf("expected conditional_function_definition, got %v", kindsOf(f.Root.Children))
	}
	n := f.Root.Children[0]
	if n.Field("headers") == nil || n.Field("body") == nil || n.HasError {
		t.Fatalf("conditional function is incomplete or erroneous: %+v", n)
	}
}

func TestDeclaratorListSplitByConditionalRegion(t *testing.T) {
	t.Parallel()
	src := "static const a[] = {1, 2},\n#if FEATURE\nb[] = {3, 4},\n#endif\nc[] = {5, 6};\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	decl := f.Root.Children[0]
	if decl.Kind != KindVariableDeclaration {
		t.Fatalf("expected variable_declaration, got %v", decl.Kind)
	}
	foundRegion := false
	for _, c := range decl.Children {
		if c.Kind == KindConditionalRegion {
			foundRegion = true
		}
	}
	if !foundRegion {
		t.Fatalf("expected a conditional_region among the declarators, got %v", kindsOf(decl.Children))
	}
}

func TestLowercaseCustomTagCastInInitializer(t *testing.T) {
	t.Parallel()
	src := "const tag_uid:tag_uid_unknown = tag_uid:0;\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 || f.Root.Children[0].Kind != KindVariableDeclaration {
		t.Fatalf("expected one variable declaration, got %v", kindsOf(f.Root.Children))
	}
	decl := f.Root.Children[0]
	if decl.HasError {
		t.Fatalf("declaration unexpectedly has an error: %+v", decl)
	}
	init := decl.Children[len(decl.Children)-1].Field("initializer")
	if init == nil || init.Kind != KindTaggedExpression || init.Text([]byte(src)) != "tag_uid:0" {
		t.Fatalf("expected lowercase tagged initializer, got %+v", init)
	}
}

func TestTagKnowledgeDoesNotConsumeTernarySeparator(t *testing.T) {
	t.Parallel()
	src := "new value = enabled ? base + offset : fallback;\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 || f.Root.Children[0].HasError {
		t.Fatalf("expected one clean declaration, got %v", kindsOf(f.Root.Children))
	}
	init := f.Root.Children[0].Children[len(f.Root.Children[0].Children)-1].Field("initializer")
	if init == nil || init.Kind != KindTernaryExpression {
		t.Fatalf("expected ternary initializer, got %+v", init)
	}
}

func TestRawRecoveryStopsAtDeclarationSemicolon(t *testing.T) {
	t.Parallel()
	src := "new bad = unknown_tag:0;\nnew good = 1;\n"
	f := Parse([]byte(src))
	if len(f.Root.Children) < 2 {
		t.Fatalf("expected recovery followed by another declaration, got %v", kindsOf(f.Root.Children))
	}
	last := f.Root.Children[len(f.Root.Children)-1]
	if last.Kind != KindVariableDeclaration || last.HasError || last.Text([]byte(src)) != "new good = 1;" {
		t.Fatalf("recovery swallowed the following declaration: %+v", last)
	}
}

func TestMultilineDeclaratorListSplitByConditionalRegion(t *testing.T) {
	t.Parallel()
	src := "new\n\t#if defined CA_RayCastLineAngle\n\t\tFloat:cX, Float:cY, Float:cZ,\n\t\tFloat:rX, Float:rY, Float:rZ,\n\t\tFloat:minZ, Float:tmp,\n\t#endif\n\tFloat:otherDeclarator;\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	if len(f.Root.Children) != 1 || f.Root.Children[0].Kind != KindVariableDeclaration {
		t.Fatalf("expected one variable declaration, got %v", kindsOf(f.Root.Children))
	}
	decl := f.Root.Children[0]
	if decl.HasError {
		t.Fatalf("declaration unexpectedly has an error: %+v", decl)
	}
	foundRegion := false
	for _, child := range decl.Children {
		if child.Kind == KindConditionalRegion {
			foundRegion = true
		}
		if child.Kind == KindRaw || child.HasError {
			t.Fatalf("unexpected raw/error child %s [%d:%d]", child.Kind, child.Start, child.End)
		}
	}
	if !foundRegion {
		t.Fatalf("expected a conditional region among declarators, got %v", kindsOf(decl.Children))
	}
}

func TestConditionalControlFlowWrapperIsPreserved(t *testing.T) {
	t.Parallel()
	src := `public HandleClick(playerid)
{
#if AD_FAST_DOUBLE_CLICK
	if(gtc - adLastClicked[playerid] <= AD_MAX_CLICK_INTERVAL)
	{
		adLastClicked[playerid] = 0;
#endif
		new tmp_dialogid = adDialogID[playerid], tmp_itemid = adItemID[playerid];
		ShowPlayerAltDialog(playerid, AD_INVALID_ID, -1, "", "", "");
		CallRemoteFunction("OnAltDialogResponse", "iiii", playerid, tmp_dialogid, 1, tmp_itemid);
#if AD_FAST_DOUBLE_CLICK
	}
	else adLastClicked[playerid] = gtc;
#endif
}
`
	f := Parse([]byte(src))
	if f == nil || f.Root == nil {
		t.Fatal("wrapper idiom must be preserved in a CST")
	}
	if got := f.Root.Text([]byte(src)); got != src {
		t.Fatalf("wrapper idiom was not preserved verbatim:\n%q", got)
	}
	if len(f.Root.Children) != 1 || f.Root.Children[0].Kind != KindFunctionDefinition {
		t.Fatalf("expected the containing function to remain structured, got %v", kindsOf(f.Root.Children))
	}
	fn := f.Root.Children[0]
	if fn.HasError || f.Root.HasError || f.Broken {
		t.Fatal("conditional splices must not be reported as invalid Pawn")
	}
	body := fn.Field("body")
	if body == nil || len(body.Children) != 5 {
		t.Fatalf("expected two splices around three shared statements, got %+v", body)
	}
	want := []Kind{KindConditionalSplice, KindVariableDeclaration, KindExpressionStatement, KindExpressionStatement, KindConditionalSplice}
	for i, child := range body.Children {
		if child.Kind != want[i] || child.HasError {
			t.Fatalf("body child %d: expected clean %s, got %+v", i, want[i], child)
		}
	}
	if got := body.Children[0].Text([]byte(src)); got != "#if AD_FAST_DOUBLE_CLICK\n\tif(gtc - adLastClicked[playerid] <= AD_MAX_CLICK_INTERVAL)\n\t{\n\t\tadLastClicked[playerid] = 0;\n#endif" {
		t.Fatalf("opening splice consumed shared source:\n%q", got)
	}
	if got := body.Children[4].Text([]byte(src)); got != "#if AD_FAST_DOUBLE_CLICK\n\t}\n\telse adLastClicked[playerid] = gtc;\n#endif" {
		t.Fatalf("closing splice was not preserved exactly:\n%q", got)
	}
}

func TestMacroQualifierFunctionPattern(t *testing.T) {
	t.Parallel()
	src := "ac_fpublic ac_DoThing(playerid)\n{\n    return playerid;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	fn := f.Root.Children[0]
	if fn.Kind != KindFunctionDefinition {
		t.Fatalf("expected function_definition, got %v", fn.Kind)
	}
	if fn.HasError {
		t.Fatalf("function unexpectedly HasError")
	}
	name := fn.Field("name")
	if name == nil || name.Text([]byte(src)) != "ac_DoThing" {
		t.Fatalf("expected function name ac_DoThing, got %+v", name)
	}
}

func TestNestedConditionalDeadBranchIgnoredAtAnyDepth(t *testing.T) {
	t.Parallel()
	src := "stock F()\n{\n" +
		"#if OUTER\n" +
		"#if defined foreach\n" +
		"foreach(new i : Player)\n{\n" +
		"#else\n" +
		"for(new i = 0; i < MAX; i++)\n" +
		"#endif\n" +
		"{\n" +
		"if (i) continue;\n" +
		"}\n" +
		"#endif\n" +
		"return 1;\n}\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
}

func TestIteratorMacroArgumentParsesWithoutRawFallback(t *testing.T) {
	t.Parallel()
	src := "stock F() { foreach(new i : Player) Use(i); }\n"
	f := Parse([]byte(src))
	mustNotBeBroken(t, f, src)
	var visit func(*Node)
	visit = func(n *Node) {
		if n.Kind == KindRaw || n.HasError {
			t.Fatalf("iterator macro argument fell back to raw/error node %s [%d:%d]", n.Kind, n.Start, n.End)
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	visit(f.Root)
}

func TestOpaqueTokenPastingMacroDoesNotPoisonParseTree(t *testing.T) {
	t.Parallel()
	src := "#if FEATURE\n#define ac_fpublic%0(%1) forward%0(%1); public%0(%1)\n#endif\n"
	f := Parse([]byte(src))
	if f.Root.HasError || f.Root.Children[0].HasError {
		t.Fatalf("valid opaque macro body propagated a parser error:\n%s", src)
	}
}

func TestOrphanedFunctionBodyRecoversWithoutMisparsingInterior(t *testing.T) {
	t.Parallel()
	src := "#if defined X\n" +
		"stock F(a)\n" +
		"#else\n" +
		"stock F(b)\n" +
		"#endif\n" +
		"{\n" +
		"#if defined Y\n" +
		"    return 1;\n" +
		"#endif\n" +
		"}\n"
	f := Parse([]byte(src))
	if f.Broken {
		t.Fatalf("parser reported broken for a hard-but-contained shape:\n%s", src)
	}
}

func kindsOf(nodes []*Node) []Kind {
	out := make([]Kind, len(nodes))
	for i, n := range nodes {
		out[i] = n.Kind
	}
	return out
}
