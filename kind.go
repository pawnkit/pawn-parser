package parser

// Kind identifies the syntactic category of a Node in the parsed CST.
type Kind uint16

const (
	// KindInvalid is the zero value of Kind, used for uninitialized nodes.
	KindInvalid Kind = iota

	KindSourceFile
	KindComment
	KindRaw

	// Directives
	KindDirectiveInclude
	KindDirectiveTryInclude
	KindDirectiveDefine
	KindDirectiveUndef
	KindDirectiveIf
	KindDirectiveElseif
	KindDirectiveElse
	KindDirectiveEndif
	KindDirectivePragma
	KindDirectiveError
	KindDirectiveWarning
	KindDirectiveEmit
	KindDirectiveAssert
	KindDirectiveLine
	KindDirectiveFile
	KindDirectiveEndinput
	KindDirectiveRaw
	KindConditionalRegion
	KindConditionalBranch
	KindSharedConditional
	KindSharedConditionalPrefix
	KindConditionalFunction

	// Declarations
	KindFunctionDefinition
	KindFunctionDeclaration
	KindVariableDeclaration
	KindVariableDeclarator
	KindEnumDeclaration
	KindEnumEntry
	KindParameterList
	KindParameter
	KindArgumentList
	KindDimension
	KindTaggedType

	// Statements
	KindBlock
	KindIfStatement
	KindWhileStatement
	KindDoWhileStatement
	KindForStatement
	KindSwitchStatement
	KindCaseClause
	KindDefaultClause
	KindCaseValueList
	KindCaseRange
	KindGotoStatement
	KindLabelStatement
	KindReturnStatement
	KindBreakStatement
	KindContinueStatement
	KindStateStatement
	KindExpressionStatement
	KindEmptyStatement
	KindMacroInvocationBlock

	// Expressions
	KindIdentifier
	KindLiteral
	KindCallExpression
	KindSubscriptExpression
	KindTernaryExpression
	KindBinaryExpression
	KindUnaryExpression
	KindUpdateExpression
	KindAssignmentExpression
	KindSizeofExpression
	KindTagofExpression
	KindDefinedExpression
	KindTaggedExpression
	KindParenthesizedExpression
	KindArrayLiteral
	KindExpressionList
	KindArgumentName
	KindIteratorArgument
	KindStringizeExpression
	KindStringConcat
	KindConditionalSplice
	KindDirectivePath
	KindMacroBody
	KindEnumIncrementClause
	KindStateSelector
	KindMacroInvocation
)

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "unknown"
}

var kindNames = map[Kind]string{
	KindInvalid:                 "invalid",
	KindSourceFile:              "source_file",
	KindComment:                 "comment",
	KindRaw:                     "raw",
	KindDirectiveInclude:        "directive_include",
	KindDirectiveTryInclude:     "directive_tryinclude",
	KindDirectiveDefine:         "directive_define",
	KindDirectiveUndef:          "directive_undef",
	KindDirectiveIf:             "directive_if",
	KindDirectiveElseif:         "directive_elseif",
	KindDirectiveElse:           "directive_else",
	KindDirectiveEndif:          "directive_endif",
	KindDirectivePragma:         "directive_pragma",
	KindDirectiveError:          "directive_error",
	KindDirectiveWarning:        "directive_warning",
	KindDirectiveEmit:           "directive_emit",
	KindDirectiveAssert:         "directive_assert",
	KindDirectiveLine:           "directive_line",
	KindDirectiveFile:           "directive_file",
	KindDirectiveEndinput:       "directive_endinput",
	KindDirectiveRaw:            "directive_raw",
	KindConditionalRegion:       "conditional_region",
	KindConditionalBranch:       "conditional_branch",
	KindConditionalSplice:       "conditional_splice",
	KindDirectivePath:           "directive_path",
	KindMacroBody:               "macro_body",
	KindEnumIncrementClause:     "enum_increment_clause",
	KindStateSelector:           "state_selector",
	KindMacroInvocation:         "macro_invocation",
	KindSharedConditional:       "shared_conditional",
	KindSharedConditionalPrefix: "shared_conditional_prefix",
	KindConditionalFunction:     "conditional_function_definition",
	KindFunctionDefinition:      "function_definition",
	KindFunctionDeclaration:     "function_declaration",
	KindVariableDeclaration:     "variable_declaration",
	KindVariableDeclarator:      "variable_declarator",
	KindEnumDeclaration:         "enum_declaration",
	KindEnumEntry:               "enum_entry",
	KindParameterList:           "parameter_list",
	KindParameter:               "parameter",
	KindArgumentList:            "argument_list",
	KindDimension:               "dimension",
	KindTaggedType:              "tagged_type",
	KindBlock:                   "block",
	KindIfStatement:             "if_statement",
	KindWhileStatement:          "while_statement",
	KindDoWhileStatement:        "do_while_statement",
	KindForStatement:            "for_statement",
	KindSwitchStatement:         "switch_statement",
	KindCaseClause:              "case_clause",
	KindDefaultClause:           "default_clause",
	KindCaseValueList:           "case_value_list",
	KindCaseRange:               "case_range",
	KindGotoStatement:           "goto_statement",
	KindLabelStatement:          "label_statement",
	KindReturnStatement:         "return_statement",
	KindBreakStatement:          "break_statement",
	KindContinueStatement:       "continue_statement",
	KindStateStatement:          "state_statement",
	KindExpressionStatement:     "expression_statement",
	KindEmptyStatement:          "empty_statement",
	KindMacroInvocationBlock:    "macro_invocation_block",
	KindIdentifier:              "identifier",
	KindLiteral:                 "literal",
	KindCallExpression:          "call_expression",
	KindSubscriptExpression:     "subscript_expression",
	KindTernaryExpression:       "ternary_expression",
	KindBinaryExpression:        "binary_expression",
	KindUnaryExpression:         "unary_expression",
	KindUpdateExpression:        "update_expression",
	KindAssignmentExpression:    "assignment_expression",
	KindSizeofExpression:        "sizeof_expression",
	KindTagofExpression:         "tagof_expression",
	KindDefinedExpression:       "defined_expression",
	KindTaggedExpression:        "tagged_expression",
	KindParenthesizedExpression: "parenthesized_expression",
	KindArrayLiteral:            "array_literal",
	KindExpressionList:          "expression_list",
	KindArgumentName:            "argument_name",
	KindIteratorArgument:        "iterator_argument",
	KindStringizeExpression:     "stringize_expression",
	KindStringConcat:            "string_concat",
}

// IsDirective reports whether k is one of the directive node kinds.
func (k Kind) IsDirective() bool {
	switch k {
	case KindDirectiveInclude, KindDirectiveTryInclude, KindDirectiveDefine,
		KindDirectiveUndef, KindDirectiveIf, KindDirectiveElseif, KindDirectiveElse,
		KindDirectiveEndif, KindDirectivePragma, KindDirectiveError, KindDirectiveWarning,
		KindDirectiveEmit, KindDirectiveAssert, KindDirectiveLine, KindDirectiveFile,
		KindDirectiveEndinput, KindDirectiveRaw:
		return true
	default:
		return false
	}
}

// IsTopLevelDeclaration reports whether k is a top-level declaration kind.
func IsTopLevelDeclaration(k Kind) bool {
	switch k {
	case KindFunctionDefinition, KindFunctionDeclaration, KindVariableDeclaration, KindEnumDeclaration:
		return true
	default:
		return false
	}
}
