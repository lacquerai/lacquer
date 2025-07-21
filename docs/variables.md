# Variable Interpolation and Outputs

Lacquer uses GitHub Actions-style template syntax with `${{ }}` for variable interpolation, allowing dynamic values throughout your workflows. This document covers variable usage, expression types, built-in functions, and best practices.

## Variable Syntax

### Basic Template Syntax

All variable interpolation in Lacquer uses the `${{ }}` syntax:

```yaml
steps:
  - id: greet
    agent: assistant
    prompt: "Hello ${{ inputs.name }}, welcome to ${{ inputs.location }}!"
```

### Variable Contexts

Lacquer provides several contexts for variable access:

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

Lacquer provides comprehensive built-in functions for data manipulation:

### String Functions

#### `contains(search, item)`

Returns true if search contains item.

**Parameters:**
- `search` (string, required): The string to search within
- `item` (string, required): The substring to look for

**Returns:** `boolean`

**Example:**
```yaml
has_world: ${{ contains("hello world", "world") }}  # → true
```

#### `startsWith(searchString, searchValue)`

Returns true if searchString starts with searchValue.

**Parameters:**
- `searchString` (string, required): The string to check
- `searchValue` (string, required): The value to check at the start

**Returns:** `boolean`

**Example:**
```yaml
starts_hello: ${{ startsWith("hello world", "hello") }}  # → true
```

#### `endsWith(searchString, searchValue)`

Returns true if searchString ends with searchValue.

**Parameters:**
- `searchString` (string, required): The string to check
- `searchValue` (string, required): The value to check at the end

**Returns:** `boolean`

**Example:**
```yaml
ends_world: ${{ endsWith("hello world", "world") }}  # → true
```

#### `format(format, ...args)`

Formats a string with placeholders using {0}, {1}, etc.

**Parameters:**
- `format` (string, required): The format string with placeholders
- `args` (any, optional): Values to substitute into placeholders

**Returns:** `string`

**Example:**
```yaml
greeting: ${{ format("Hello {0}!", "world") }}  # → "Hello world!"
formatted_message: ${{ format("{0} has {1} items", inputs.user, inputs.count) }}
```

#### `length(value)`

Returns the length of an array, string, or object.

**Parameters:**
- `value` (any, required): The value to measure

**Returns:** `number`

**Example:**
```yaml
text_length: ${{ length("hello") }}  # → 5
array_length: ${{ length([1, 2, 3]) }}  # → 3
```

### Array Functions

#### `join(array, separator)`

Joins array elements with separator.

**Parameters:**
- `array` (array, required): The array to join
- `separator` (string, optional): The separator to use (defaults to comma)

**Returns:** `string`

**Example:**
```yaml
csv_list: ${{ join(["a", "b", "c"], "-") }}  # → "a-b-c"
default_join: ${{ join(inputs.items) }}  # Uses comma by default
```

### Object Functions

#### `keys(object)`

Returns the keys of an object as an array.

**Parameters:**
- `object` (object, required): The object to get keys from

**Returns:** `array`

**Example:**
```yaml
object_keys: ${{ keys({a: 1, b: 2}) }}  # → ["a", "b"]
```

#### `values(object)`

Returns the values of an object as an array.

**Parameters:**
- `object` (object, required): The object to get values from

**Returns:** `array`

**Example:**
```yaml
object_values: ${{ values({a: 1, b: 2}) }}  # → [1, 2]
```

### JSON Functions

#### `toJSON(value)`

Converts value to JSON string.

**Parameters:**
- `value` (any, required): The value to convert to JSON

**Returns:** `string`

**Example:**
```yaml
json_data: ${{ toJSON({name: "test"}) }}  # → '{"name":"test"}'
```

#### `fromJSON(jsonString)`

Parses JSON string to object.

**Parameters:**
- `jsonString` (string, required): The JSON string to parse

**Returns:** `object`

**Example:**
```yaml
parsed_data: ${{ fromJSON('{"name":"test"}') }}  # → {name: "test"}
```

### File System Functions

#### `hashFiles(...paths)`

Returns MD5 hash of the specified files.

**Parameters:**
- `paths` (string, required): File paths to hash

**Returns:** `string`

**Example:**
```yaml
file_hash: ${{ hashFiles("package.json", "yarn.lock") }}  # → "abc123..."
```

#### `glob(pattern)`

Returns files matching the specified glob pattern.

**Parameters:**
- `pattern` (string, required): The glob pattern to match

**Returns:** `array`

**Example:**
```yaml
js_files: ${{ glob("*.js") }}  # → ["file1.js", "file2.js"]
all_configs: ${{ glob("**/*.config.*") }}
```

### Workflow Status Functions

#### `always()`

Always returns true, regardless of previous step status.

**Returns:** `boolean`

**Example:**
```yaml
always_true: ${{ always() }}  # → true
```

#### `cancelled()`

Returns true if workflow was cancelled.

**Returns:** `boolean`

**Example:**
```yaml
is_cancelled: ${{ cancelled() }}  # → false
```

#### `failure()`

Returns true if any previous step failed.

**Returns:** `boolean`

**Example:**
```yaml
has_failure: ${{ failure() }}  # → false
```

#### `success()`

Returns true if no previous step failed.

**Returns:** `boolean`

**Example:**
```yaml
is_success: ${{ success() }}  # → true
```

## Best Practices

1. **Use meaningful variable names**
2. **Provide defaults for optional values**
3. **Handle null/undefined values gracefully**

## Next Steps

- Explore [Examples](./examples/) to see variables in action
- Learn about [Workflow Steps](./workflow-steps.md) for more context
- Check [State Management](./state-management.md) for state handling