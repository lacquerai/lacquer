package schema

// JSON represents a JSON Schema object
type JSON struct {
	// Core vocabulary generally not used in Lacquer
	// @TODO: remove these once we know we don't need them
	Schema        string          `json:"$schema,omitempty"`
	ID            string          `json:"$id,omitempty"`
	Ref           string          `json:"$ref,omitempty"`
	Anchor        string          `json:"$anchor,omitempty"`
	DynamicRef    string          `json:"$dynamicRef,omitempty"`
	DynamicAnchor string          `json:"$dynamicAnchor,omitempty"`
	Definitions   map[string]JSON `json:"$defs,omitempty"`
	Comment       string          `json:"$comment,omitempty"`

	// Type constraints
	Type  interface{}   `json:"type,omitempty"`
	Enum  []interface{} `json:"enum,omitempty"`
	Const interface{}   `json:"const,omitempty"`

	// String validation
	MinLength int    `json:"minLength,omitempty"`
	MaxLength int    `json:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Format    string `json:"format,omitempty"`

	// Numeric validation
	MultipleOf       float64 `json:"multipleOf,omitempty"`
	Minimum          float64 `json:"minimum,omitempty"`
	ExclusiveMinimum float64 `json:"exclusiveMinimum,omitempty"`
	Maximum          float64 `json:"maximum,omitempty"`
	ExclusiveMaximum float64 `json:"exclusiveMaximum,omitempty"`

	// Object validation
	Properties           map[string]JSON `json:"properties,omitempty"`
	PatternProperties    map[string]JSON `json:"patternProperties,omitempty"`
	AdditionalProperties interface{}     `json:"additionalProperties,omitempty"` // bool or JSON
	Required             []string        `json:"required,omitempty"`
	PropertyNames        *JSON           `json:"propertyNames,omitempty"`
	MinProperties        int             `json:"minProperties,omitempty"`
	MaxProperties        int             `json:"maxProperties,omitempty"`

	// Array validation
	Items       interface{} `json:"items,omitempty"` // JSON or bool
	PrefixItems []JSON      `json:"prefixItems,omitempty"`
	Contains    *JSON       `json:"contains,omitempty"`
	MinContains int         `json:"minContains,omitempty"`
	MaxContains int         `json:"maxContains,omitempty"`
	MinItems    int         `json:"minItems,omitempty"`
	MaxItems    int         `json:"maxItems,omitempty"`
	UniqueItems bool        `json:"uniqueItems,omitempty"`

	// Combining schemas
	AllOf []JSON `json:"allOf,omitempty"`
	AnyOf []JSON `json:"anyOf,omitempty"`
	OneOf []JSON `json:"oneOf,omitempty"`
	Not   *JSON  `json:"not,omitempty"`

	// Metadata
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
	ReadOnly    bool          `json:"readOnly,omitempty"`
	WriteOnly   bool          `json:"writeOnly,omitempty"`
	Examples    []interface{} `json:"examples,omitempty"`
}
