package parser

// FieldID identifies a named syntax field.
type FieldID uint8

const (
	fieldInvalid FieldID = iota
	fieldAlias
	fieldAlternative
	fieldArguments
	fieldArray
	fieldBody
	fieldCallingConvention
	fieldCapacity
	fieldCondition
	fieldConditionalAlternatives
	fieldConsequence
	fieldDefaultValue
	fieldDimensions
	fieldDirective
	fieldEnd
	fieldExpression
	fieldFunction
	fieldGeneric
	fieldHeaders
	fieldIncrement
	fieldIndex
	fieldInit
	fieldInitializer
	fieldLabel
	fieldLeft
	fieldName
	fieldPacked
	fieldParameters
	fieldPath
	fieldPrefix
	fieldRight
	fieldSharedAlternative
	fieldSize
	fieldStart
	fieldState
	fieldStorage
	fieldTag
	fieldTarget
	fieldValue
	fieldValues
)

var fieldNames = [...]string{
	fieldInvalid:                 "",
	fieldAlias:                   "alias",
	fieldAlternative:             "alternative",
	fieldArguments:               "arguments",
	fieldArray:                   "array",
	fieldBody:                    "body",
	fieldCallingConvention:       "calling_convention",
	fieldCapacity:                "capacity",
	fieldCondition:               "condition",
	fieldConditionalAlternatives: "conditional_alternatives",
	fieldConsequence:             "consequence",
	fieldDefaultValue:            "default_value",
	fieldDimensions:              "dimensions",
	fieldDirective:               "directive",
	fieldEnd:                     "end",
	fieldExpression:              "expression",
	fieldFunction:                "function",
	fieldGeneric:                 "generic",
	fieldHeaders:                 "headers",
	fieldIncrement:               "increment",
	fieldIndex:                   "index",
	fieldInit:                    "init",
	fieldInitializer:             "initializer",
	fieldLabel:                   "label",
	fieldLeft:                    "left",
	fieldName:                    "name",
	fieldPacked:                  "packed",
	fieldParameters:              "parameters",
	fieldPath:                    "path",
	fieldPrefix:                  "prefix",
	fieldRight:                   "right",
	fieldSharedAlternative:       "shared_alternative",
	fieldSize:                    "size",
	fieldStart:                   "start",
	fieldState:                   "state",
	fieldStorage:                 "storage",
	fieldTag:                     "tag",
	fieldTarget:                  "target",
	fieldValue:                   "value",
	fieldValues:                  "values",
}

// String returns the field name.
func (id FieldID) String() string {
	if int(id) >= len(fieldNames) {
		return ""
	}
	return fieldNames[id]
}

func lookupFieldID(name string) FieldID {
	for id, candidate := range fieldNames[1:] {
		if candidate == name {
			return FieldID(id + 1)
		}
	}
	return fieldInvalid
}
