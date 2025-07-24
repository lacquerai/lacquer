# Features

Lacquer's core features are provided by the `laq` command line tool.

## `laq init`

Initialize a new Lacquer project with interactive setup. 

```bash
laq init
```

This will create a new directory with a `workflow.laq.yaml` based on your answers to the prompts. We use a fine tuned model to generate the workflow based on your answers.

## `laq run`

Run a Lacquer workflow.

```bash
laq run workflow.laq.yaml
```

This will run the workflow and print the output to the console.

### Configuration Options

- `--config` - Config file (default is $HOME/.lacquer/config.yaml)
- `-help` - Help for run
- `--input` - Input parameters (key=value)
- `--input-file` - Input parameters from file
- `--input-json` - Input parameters as JSON
- `--output` - Output format (text, json, yaml)
- `--timeout` - Overall execution timeout

### Examples

```bash
# Run a workflow with input parameters
laq run workflow.laq.yaml --input "name=John"

# Run a workflow with input parameters from a file
laq run workflow.laq.yaml --input-file inputs.json

# Run a workflow with input parameters as JSON
laq run workflow.laq.yaml --input-json '{"name": "John"}'

# Run a workflow with output as json and pipe to jq
laq run workflow.laq.yaml --output output.json | jq
```

## `laq validate`

Validate a Lacquer workflow.

```bash
laq validate workflow.laq.yaml
```

This will validate the workflow and print the output to the console.

## `laq serve`

Start a HTTP server for Lacquer workflow executions

```bash
laq serve workflow.laq.yaml ...
```

This will start a HTTP server that provides a REST API for executing Lacquer workflows with real-time progress updates via WebSocket streaming.

### Configuration Options

- `--concurrency` - Maximum concurrent executions (default: 5)
- `--timeout` - Default execution timeout (default: 30m)
- `--workflow-dir` - Directory containing workflow files
- `--metrics` - Enable Prometheus metrics endpoint (default: true)
- `--cors` - Enable CORS headers (default: true)

### Examples

```bash
# Serve a single workflow
laq serve workflow.laq.yaml

# Serve multiple workflows
laq serve workflow1.laq.yaml workflow2.laq.yaml

# Serve all workflows in a directory
laq serve --workflow-dir ./workflows

# Custom host and port with higher concurrency
laq serve --port 8080 --host 0.0.0.0 --concurrency 10 workflow.laq.yaml
```

### REST API Endpoints

#### List Workflows
```
GET /api/v1/workflows
```

Returns all available workflows with their metadata.

**Response:**
```json
{
  "workflows": {
    "workflow-id": {
      "version": "1.0",
      "name": "Workflow Name",
      "description": "Workflow description",
      "steps": 5
    }
  }
}
```

#### Execute Workflow
```
POST /api/v1/workflows/{id}/execute
```

Starts a workflow execution with optional input parameters.

**Request Body:**
```json
{
  "inputs": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

**Response:**
```json
{
  "run_id": "execution-uuid",
  "workflow_id": "workflow-id",
  "status": "running",
  "started_at": "2024-01-01T12:00:00Z"
}
```

#### Get Execution Status
```
GET /api/v1/executions/{runId}
```

Returns the current status and results of a workflow execution.

**Response:**
```json
{
  "run_id": "execution-uuid",
  "workflow_id": "workflow-id",
  "status": "completed|running|failed",
  "start_time": "2024-01-01T12:00:00Z",
  "end_time": "2024-01-01T12:05:00Z",
  "duration": 300000000000,
  "inputs": { "param1": "value1" },
  "outputs": { "result": "output value" },
  "error": "error message if failed",
  "progress": []
}
```

#### Stream Execution Progress
```
WebSocket: /api/v1/workflows/{id}/stream?run_id={runId}
```

Provides real-time streaming of workflow execution progress via WebSocket. Events are sent as JSON messages containing step updates, completions, and errors.

### Additional Endpoints

#### Health Check
```
GET /health
```

Returns server health status and metrics.

**Response:**
```json
{
  "status": "healthy",
  "workflows_loaded": 3,
  "active_executions": 2,
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### Metrics (if enabled)
```
GET /metrics
```

Returns Prometheus metrics for monitoring server performance and workflow execution statistics.

