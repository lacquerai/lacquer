# test-project

A basic Lacquer workflow project.

## Getting Started

1. **Validate the workflow:**
```bash
   laq validate workflow.laq.yaml
```

2. **Run the workflow:**
```bash
   laq run workflow.laq.yaml --input topic="machine learning"
```

3. **Customize the workflow:**
   - Edit `workflow.laq.yaml` to modify the conversation
   - Add new agents with different models or configurations
   - Create additional steps for more complex interactions

## Configuration

The project configuration is stored in `.lacquer/config.yaml`. You can customize:
- Default model settings
- API keys and credentials
- Logging preferences

## Learn More

- [Lacquer Documentation](https://lacquer.ai/docs)
- [DSL Reference](https://lacquer.ai/docs/dsl)
- [Examples](https://lacquer.ai/examples)
