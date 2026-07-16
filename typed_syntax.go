package parser

// Declarations iterates top-level declarations.
func (n SyntaxNode) Declarations() DeclarationIterator {
	return DeclarationIterator{children: n.Children()}
}

// DeclarationIterator iterates top-level declarations without allocating.
type DeclarationIterator struct {
	children SyntaxIterator
	current  SyntaxNode
}

// Next advances to the next declaration.
func (i *DeclarationIterator) Next() bool {
	for i != nil && i.children.Next() {
		node := i.children.Node()
		if IsTopLevelDeclaration(node.Kind()) {
			i.current = node
			return true
		}
	}
	return false
}

// Declaration returns the current declaration.
func (i *DeclarationIterator) Declaration() SyntaxNode {
	if i == nil {
		return SyntaxNode{}
	}
	return i.current
}

// FunctionSyntax is a typed function declaration or definition handle.
type FunctionSyntax struct{ SyntaxNode }

// AsFunction converts n when it is a function node.
func AsFunction(n SyntaxNode) (FunctionSyntax, bool) {
	ok := n.Kind() == KindFunctionDefinition || n.Kind() == KindFunctionDeclaration
	return FunctionSyntax{n}, ok
}

// Name returns the function name.
func (f FunctionSyntax) Name() (SyntaxNode, bool) { return f.field(fieldName) }

// Parameters returns the function parameter list.
func (f FunctionSyntax) Parameters() ParameterIterator {
	list, ok := f.field(fieldParameters)
	if !ok {
		return ParameterIterator{}
	}
	return ParameterIterator{children: list.Children()}
}

// Body returns the function body when present.
func (f FunctionSyntax) Body() (BlockSyntax, bool) {
	node, ok := f.field(fieldBody)
	if !ok || node.Kind() != KindBlock {
		return BlockSyntax{}, false
	}
	return BlockSyntax{node}, true
}

// ParameterSyntax is a typed parameter handle.
type ParameterSyntax struct{ SyntaxNode }

// Name returns the parameter name.
func (p ParameterSyntax) Name() (SyntaxNode, bool) { return p.field(fieldName) }

// DefaultValue returns the default value when present.
func (p ParameterSyntax) DefaultValue() (ExpressionSyntax, bool) {
	node, ok := p.field(fieldDefaultValue)
	return ExpressionSyntax{node}, ok
}

// ParameterIterator iterates parameters without allocating.
type ParameterIterator struct {
	children SyntaxIterator
	current  ParameterSyntax
}

// Next advances to the next parameter.
func (i *ParameterIterator) Next() bool {
	for i != nil && i.children.Next() {
		node := i.children.Node()
		if node.Kind() == KindParameter {
			i.current = ParameterSyntax{node}
			return true
		}
	}
	return false
}

// Parameter returns the current parameter.
func (i *ParameterIterator) Parameter() ParameterSyntax {
	if i == nil {
		return ParameterSyntax{}
	}
	return i.current
}

// BlockSyntax is a typed block statement handle.
type BlockSyntax struct{ SyntaxNode }

// Statements iterates statements in the block.
func (b BlockSyntax) Statements() SyntaxIterator { return b.Children() }

// IfSyntax is a typed if statement handle.
type IfSyntax struct{ SyntaxNode }

// AsIf converts n when it is an if statement.
func AsIf(n SyntaxNode) (IfSyntax, bool) {
	return IfSyntax{n}, n.Kind() == KindIfStatement
}

// Condition returns the if condition.
func (s IfSyntax) Condition() (ExpressionSyntax, bool) {
	node, ok := s.field(fieldCondition)
	return ExpressionSyntax{node}, ok
}

// Consequence returns the if consequence.
func (s IfSyntax) Consequence() (StatementSyntax, bool) {
	node, ok := s.field(fieldConsequence)
	return StatementSyntax{node}, ok
}

// Alternative returns the else branch when present.
func (s IfSyntax) Alternative() (StatementSyntax, bool) {
	node, ok := s.field(fieldAlternative)
	return StatementSyntax{node}, ok
}

// LoopSyntax is a typed loop statement handle.
type LoopSyntax struct{ SyntaxNode }

// AsLoop converts while, do-while, and for nodes.
func AsLoop(n SyntaxNode) (LoopSyntax, bool) {
	switch n.Kind() {
	case KindWhileStatement, KindDoWhileStatement, KindForStatement:
		return LoopSyntax{n}, true
	default:
		return LoopSyntax{n}, false
	}
}

// Condition returns the loop condition when present.
func (s LoopSyntax) Condition() (ExpressionSyntax, bool) {
	node, ok := s.field(fieldCondition)
	return ExpressionSyntax{node}, ok
}

// Body returns the loop body.
func (s LoopSyntax) Body() (StatementSyntax, bool) {
	node, ok := s.field(fieldBody)
	return StatementSyntax{node}, ok
}

// CallSyntax is a typed call expression handle.
type CallSyntax struct{ SyntaxNode }

// AsCall converts n when it is a call expression.
func AsCall(n SyntaxNode) (CallSyntax, bool) {
	return CallSyntax{n}, n.Kind() == KindCallExpression
}

// Function returns the called expression.
func (c CallSyntax) Function() (ExpressionSyntax, bool) {
	node, ok := c.field(fieldFunction)
	return ExpressionSyntax{node}, ok
}

// Arguments returns the argument nodes.
func (c CallSyntax) Arguments() SyntaxIterator {
	list, ok := c.field(fieldArguments)
	if !ok {
		return SyntaxIterator{}
	}
	return list.Children()
}

// VariableSyntax is a typed variable declarator handle.
type VariableSyntax struct{ SyntaxNode }

// AsVariable converts n when it is a variable declarator.
func AsVariable(n SyntaxNode) (VariableSyntax, bool) {
	return VariableSyntax{n}, n.Kind() == KindVariableDeclarator
}

// Name returns the variable name.
func (v VariableSyntax) Name() (SyntaxNode, bool) { return v.field(fieldName) }

// Initializer returns the variable initializer when present.
func (v VariableSyntax) Initializer() (ExpressionSyntax, bool) {
	node, ok := v.field(fieldInitializer)
	return ExpressionSyntax{node}, ok
}

// EnumSyntax is a typed enum declaration handle.
type EnumSyntax struct{ SyntaxNode }

// AsEnum converts n when it is an enum declaration.
func AsEnum(n SyntaxNode) (EnumSyntax, bool) {
	return EnumSyntax{n}, n.Kind() == KindEnumDeclaration
}

// Name returns the enum name when present.
func (e EnumSyntax) Name() (SyntaxNode, bool) { return e.field(fieldName) }

// Entries iterates enum entries.
func (e EnumSyntax) Entries() SyntaxIterator { return e.Children() }

// StatementSyntax is a lightweight statement handle.
type StatementSyntax struct{ SyntaxNode }

// ExpressionSyntax is a lightweight expression handle.
type ExpressionSyntax struct{ SyntaxNode }

// ReturnSyntax is a typed return statement handle.
type ReturnSyntax struct{ SyntaxNode }

// AsReturn converts n when it is a return statement.
func AsReturn(n SyntaxNode) (ReturnSyntax, bool) {
	return ReturnSyntax{n}, n.Kind() == KindReturnStatement
}

// Expression returns the returned expression when present.
func (r ReturnSyntax) Expression() (ExpressionSyntax, bool) {
	node, ok := r.field(fieldValue)
	return ExpressionSyntax{node}, ok
}

// BinarySyntax is a typed binary or assignment expression handle.
type BinarySyntax struct{ SyntaxNode }

// AsBinary converts binary and assignment nodes.
func AsBinary(n SyntaxNode) (BinarySyntax, bool) {
	ok := n.Kind() == KindBinaryExpression || n.Kind() == KindAssignmentExpression
	return BinarySyntax{n}, ok
}

// Left returns the left operand.
func (b BinarySyntax) Left() (ExpressionSyntax, bool) {
	node, ok := b.field(fieldLeft)
	return ExpressionSyntax{node}, ok
}

// Right returns the right operand.
func (b BinarySyntax) Right() (ExpressionSyntax, bool) {
	node, ok := b.field(fieldRight)
	return ExpressionSyntax{node}, ok
}

// UnarySyntax is a typed unary expression handle.
type UnarySyntax struct{ SyntaxNode }

// AsUnary converts unary and update nodes.
func AsUnary(n SyntaxNode) (UnarySyntax, bool) {
	ok := n.Kind() == KindUnaryExpression || n.Kind() == KindUpdateExpression
	return UnarySyntax{n}, ok
}

// Expression returns the unary operand.
func (u UnarySyntax) Expression() (ExpressionSyntax, bool) {
	node, ok := u.field(fieldExpression)
	return ExpressionSyntax{node}, ok
}

// SubscriptSyntax is a typed array access handle.
type SubscriptSyntax struct{ SyntaxNode }

// AsSubscript converts n when it is a subscript expression.
func AsSubscript(n SyntaxNode) (SubscriptSyntax, bool) {
	return SubscriptSyntax{n}, n.Kind() == KindSubscriptExpression
}

// Array returns the indexed expression.
func (s SubscriptSyntax) Array() (ExpressionSyntax, bool) {
	node, ok := s.field(fieldArray)
	return ExpressionSyntax{node}, ok
}

// Index returns the index expression when present.
func (s SubscriptSyntax) Index() (ExpressionSyntax, bool) {
	node, ok := s.field(fieldIndex)
	return ExpressionSyntax{node}, ok
}

// DirectiveSyntax is a typed preprocessor directive handle.
type DirectiveSyntax struct{ SyntaxNode }

// AsDirective converts n when it is a directive.
func AsDirective(n SyntaxNode) (DirectiveSyntax, bool) {
	return DirectiveSyntax{n}, n.Kind().IsDirective()
}

// ConditionalSyntax is a typed conditional region handle.
type ConditionalSyntax struct{ SyntaxNode }

// AsConditional converts conditional region nodes.
func AsConditional(n SyntaxNode) (ConditionalSyntax, bool) {
	switch n.Kind() {
	case KindConditionalRegion, KindSharedConditional, KindConditionalFunction:
		return ConditionalSyntax{n}, true
	default:
		return ConditionalSyntax{n}, false
	}
}

// Branches iterates conditional branches.
func (c ConditionalSyntax) Branches() SyntaxIterator { return c.Children() }
