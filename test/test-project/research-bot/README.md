# research-bot

A research-focused Lacquer workflow with web search capabilities.

## Features

- Web search integration
- Multi-step research process
- Comprehensive analysis and summarization

## Usage

```bash
# Research a topic
laq run workflow.laq.yaml --input research_topic="artificial intelligence trends 2024"

# Validate before running
laq validate workflow.laq.yaml
```

## Requirements

This workflow uses the `lacquer/web-search@v1` block, which requires:
- Internet connection
- Search API configuration (see .lacquer/config.yaml)

## Customization

- Modify agent prompts for different research styles
- Add more analysis steps
- Include fact-checking or source verification steps
- Export results to different formats
