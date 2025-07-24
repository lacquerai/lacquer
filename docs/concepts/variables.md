# Variable Interpolation

Lacquer uses GitHub Actions-style template syntax with `${{ }}` for variable interpolation, allowing dynamic values throughout your workflows. This powerful feature enables you to create flexible, reusable workflows that adapt to different inputs and conditions.

## Variable Syntax

### Basic Template Syntax

All variable interpolation in Lacquer uses the `${{ }}` syntax:

```yaml
steps:
  - id: greet
    agent: assistant
    prompt: "Hello ${{ inputs.name }}, welcome to ${{ inputs.location }}!"
```

## Variable Contexts

Lacquer provides several contexts for accessing different types of data:

```yaml
steps:
  - id: example
    agent: assistant
    prompt: |
      # Workflow inputs
      Input value: ${{ inputs.some_value }}
      
      # Step outputs
      Previous result: ${{ steps.previous_step.output }}
      Specific output: ${{ steps.analyzer.outputs.score }}
      
      # Workflow state
      Counter: ${{ state.counter }}
      Status: ${{ state.current_status }}
```

## Expression Types

Lacquer supports various expression types within the `${{ }}` syntax:

### Literal Values

Use literal values like numbers, strings, booleans, or null:

```yaml
examples:
  - Number: ${{ 42 }}
  - Float: ${{ 3.14 }}
  - String: ${{ "hello" }}
  - Boolean: ${{ true }}
  - Null: ${{ null }}
```

### Variable References

Access variables using their names:

```yaml
examples:
  - Input: ${{ inputs.name }}
  - State: ${{ state.counter }}
  - Step output: ${{ steps.step1.output }}
```

### Property Access (Dot Notation)

Access object properties using dot notation:

```yaml
examples:
  - Nested input: ${{ inputs.customer.name }}
  - Deep nesting: ${{ inputs.customer.contact.email }}
  - Step property: ${{ steps.analyzer.outputs.score }}
```

### Index Access

Access array elements by index or object properties by key:

```yaml
examples:
  - Array element: ${{ inputs.my_list[0] }}
  - Object key: ${{ steps.step_foo.outputs.map["key"] }}
  - Dynamic key: ${{ inputs.data[inputs.key_name] }}
```

### Binary Operations

Combine expressions with operators:

#### Arithmetic Operations
```yaml
examples:
  - Addition: ${{ 42 + 10 }}
  - String concat: ${{ "hello" + "world" }}
  - Multiplication: ${{ inputs.price * inputs.quantity }}
  - Division: ${{ inputs.total / inputs.count }}
  - Modulo: ${{ inputs.number % 2 }}
```

#### Comparison Operations
```yaml
examples:
  - Equality: ${{ inputs.status == "active" }}
  - Inequality: ${{ inputs.count != 0 }}
  - Greater than: ${{ inputs.score > 70 }}
  - Less than: ${{ inputs.age < 18 }}
  - Greater/equal: ${{ inputs.score >= 70 }}
  - Less/equal: ${{ inputs.age <= 65 }}
```

#### Logical Operations
```yaml
examples:
  - AND: ${{ inputs.active && inputs.verified }}
  - OR: ${{ inputs.premium || inputs.vip }}
  - Complex: ${{ inputs.falsey && steps.step_foo.outputs.age >= 18 }}
```

### Unary Operations

Apply single operators to expressions:

```yaml
examples:
  - Logical NOT: ${{ !inputs.active }}
  - Numeric negation: ${{ -inputs.count }}
  - Double negation: ${{ !!inputs.maybe_null }}
```

### Conditional (Ternary) Expressions

Use `condition ? trueValue : falseValue` for conditional logic:

```yaml
examples:
  - Simple: ${{ steps.step_foo.outputs.age >= 18 ? "adult" : "minor" }}
  - With functions: ${{ length(inputs.items) > 0 ? inputs.items : "none" }}
  - Nested: ${{ inputs.type == "premium" ? "VIP access" : inputs.verified ? "standard access" : "limited access" }}
```

### Function Calls

Call built-in functions with arguments:

```yaml
examples:
  - Length check: ${{ length(inputs.my_list) }}
  - String search: ${{ contains(inputs.text, "search") }}
  - Array join: ${{ join(inputs.items, ", ") }}
```

## Built-in Functions

Lacquer provides a comprehensive set of built-in functions for data manipulation and workflow control.

### String Functions

#### contains(search, item)

Checks if a string contains a substring.

- **Parameters**: `search` (string), `item` (string)
- **Returns**: boolean
- **Example**: `${{ contains("hello world", "world") }}` → `true`

#### startsWith(searchString, searchValue)

Checks if a string starts with a specific value.

- **Parameters**: `searchString` (string), `searchValue` (string)
- **Returns**: boolean
- **Example**: `${{ startsWith("hello world", "hello") }}` → `true`

#### endsWith(searchString, searchValue)

Checks if a string ends with a specific value.

- **Parameters**: `searchString` (string), `searchValue` (string)
- **Returns**: boolean
- **Example**: `${{ endsWith("hello world", "world") }}` → `true`

#### format(format, ...args)

Formats a string with placeholders using {0}, {1}, etc.

- **Parameters**: `format` (string), `args` (any number of values)
- **Returns**: string
- **Example**: `${{ format("Hello {0}!", "world") }}` → `"Hello world!"`

#### length(value)

Returns the length of a string, array, or number of object keys.

- **Parameters**: `value` (string | array | object)
- **Returns**: number
- **Example**: `${{ length("hello") }}` → `5`

### Array Functions

#### join(array, separator)

Joins array elements into a string.

- **Parameters**: `array` (array), `separator` (string, optional - defaults to ",")
- **Returns**: string
- **Example**: `${{ join(["a", "b", "c"], "-") }}` → `"a-b-c"`

### Object Functions

#### keys(object)

Returns an array of object keys.

- **Parameters**: `object` (object)
- **Returns**: array
- **Example**: `${{ keys({a: 1, b: 2}) }}` → `["a", "b"]`

#### values(object)

Returns an array of object values.

- **Parameters**: `object` (object)
- **Returns**: array
- **Example**: `${{ values({a: 1, b: 2}) }}` → `[1, 2]`

### JSON Functions

#### toJSON(value)

Converts a value to a JSON string.

- **Parameters**: `value` (any)
- **Returns**: string
- **Example**: `${{ toJSON({name: "test"}) }}` → `'{"name":"test"}'`

#### fromJSON(jsonString)

Parses a JSON string to an object.

- **Parameters**: `jsonString` (string)
- **Returns**: any
- **Example**: `${{ fromJSON('{"name":"test"}') }}` → `{name: "test"}`

### File System Functions

#### hashFiles(...paths)

Calculates MD5 hash of specified files.

- **Parameters**: `paths` (one or more file paths)
- **Returns**: string
- **Example**: `${{ hashFiles("package.json", "yarn.lock") }}` → `"abc123..."`

#### glob(pattern)

Finds files matching a glob pattern.

- **Parameters**: `pattern` (string)
- **Returns**: array
- **Example**: `${{ glob("*.js") }}` → `["file1.js", "file2.js"]`

### Workflow Status Functions

#### always()

Always returns true (useful for cleanup steps).

- **Parameters**: none
- **Returns**: boolean
- **Example**: `${{ always() }}` → `true`

#### cancelled()

Checks if the workflow was cancelled.

- **Parameters**: none
- **Returns**: boolean
- **Example**: `${{ cancelled() }}` → `false`

#### failure()

Checks if any previous step failed.

- **Parameters**: none
- **Returns**: boolean
- **Example**: `${{ failure() }}` → `false`

#### success()

Checks if all previous steps succeeded.

- **Parameters**: none
- **Returns**: boolean
- **Example**: `${{ success() }}` → `true`

## Related Documentation

- [Workflow Steps](workflow-steps.md) - Using variables in steps
- [State Management](state-management.md) - Working with state variables
- [Control Flow](control-flow.md) - Variables in conditions
- [Examples](examples/) - See variables in action