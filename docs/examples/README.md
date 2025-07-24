# Lacquer Workflow Examples

## Simple hello world

```yaml
# The simplest Lacquer workflow - Hello World
version: "1.0"
metadata:
  name: hello-world
  description: A simple greeting workflow demonstrating basic Lacquer syntax

# Define a single agent using GPT-4
agents:
  greeter:
    provider: openai
    model: gpt-4
    temperature: 0.7
    system_prompt: You are a friendly assistant who gives warm greetings.

# Define input parameters at root level
inputs:
  name:
    type: string
    description: Name of the person to greet
    default: "World"

# Define the workflow
workflow:
  # Single step workflow
  steps:
    - id: say_hello
      agent: greeter
      prompt: |
        Say hello to ${{ inputs.name }} in a creative and friendly way.
        Make it warm and welcoming!
  
  # Return the greeting as output
  outputs:
    greeting: ${{ steps.say_hello.output }}
```

## Content generator

```yaml
# Content Generation Workflow with Tool Integration
# This workflow demonstrates:
# - Tool integration for extended capabilities
# - Multi-step content creation process
# - Quality gates and iterative improvement
# - Error handling and fallback strategies

version: "1.0"
metadata:
  name: content-generator
  description: |
    Creates high-quality content using research tools, multiple drafts,
    and quality checks. Shows tool integration and iterative workflows.
  author: examples@lacquer.ai
  tags:
    - content-creation
    - tools
    - quality-control
  version: 2.0.0

# Configure agents with specialized roles and tools
agents:
  # Research agent with web search capability
  researcher:
    provider: openai
    model: gpt-4
    temperature: 0.2
    system_prompt: |
      You are a thorough researcher who:
      - Finds credible, recent sources
      - Identifies key facts and trends
      - Cross-references information
      - Cites all sources properly
    tools:
      - name: web_search
        script: "python3 ./tools/web_search.py"
        description: "Search the web for current information on any topic"
        parameters:
          type: object
          required: ["query"]
          properties:
            query:
              type: string
              description: "Search query - be specific and use relevant keywords"
            max_results:
              type: integer
              description: "Maximum number of results to return"
              default: 5
  
  # Content writer with style adaptation
  writer:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: |
      You are a skilled content writer who:
      - Adapts tone and style to the audience
      - Creates engaging, well-structured content
      - Uses research effectively
      - Follows SEO best practices when requested
    tools:
      - name: grammar_check
        script: "node ./tools/grammar_check.js"
        description: "Check grammar and style issues"
        parameters:
          type: object
          required: ["text"]
          properties:
            text:
              type: string
              description: "Text to check"
            style:
              type: string
              description: "Writing style guide to follow"
              enum: ["formal", "casual", "technical", "marketing"]
  
  # Editor for quality control
  editor:
    provider: openai
    model: gpt-4
    temperature: 0.1
    system_prompt: |
      You are a meticulous editor who:
      - Checks factual accuracy
      - Improves clarity and flow
      - Ensures consistent tone
      - Rates content quality objectively

# Workflow inputs
inputs:
  topic:
    type: string
    description: The topic to write about
    required: true
  
  content_type:
    type: string
    description: Type of content to create
    enum: ["blog_post", "article", "social_media", "email", "technical_doc"]
    default: "blog_post"
  
  target_audience:
    type: string
    description: Primary audience for the content
    default: "general"
  
  word_count:
    type: integer
    description: Target word count
    default: 800
  
  tone:
    type: string
    description: Desired tone of voice
    enum: ["professional", "casual", "friendly", "authoritative", "conversational"]
    default: "professional"
  
  include_seo:
    type: boolean
    description: Whether to optimize for search engines
    default: true
  
  keywords:
    type: array
    description: SEO keywords to include (if applicable)
    default: []

# Main workflow
workflow:
  # Track content creation progress
  state:
    research_complete: false
    draft_version: 0
    quality_score: 0
    sources: []
    revisions_made: []
    final_content: null
  
  steps:
    # Step 1: Research the topic
    - id: research_topic
      agent: researcher
      prompt: |
        Research "${{ inputs.topic }}" for a ${{ inputs.content_type }}.
        Target audience: ${{ inputs.target_audience }}
        
        Find:
        1. Current trends and recent developments
        2. Key statistics and facts
        3. Expert opinions
        4. Common questions/concerns
        ${{ inputs.include_seo ? '5. Search volume and competition for keywords: ' + join(inputs.keywords, ', ') : '' }}
        
        Use web search to find recent, authoritative sources.
      outputs:
        findings:
          type: array
        sources:
          type: array
        key_points:
          type: array
        statistics:
          type: array
      updates:
        research_complete: true
        sources: ${{ steps.research_topic.outputs.sources }}
    
    # Step 2: Create content outline
    - id: create_outline
      agent: writer
      prompt: |
        Create a detailed outline for a ${{ inputs.content_type }} about "${{ inputs.topic }}".
        
        Research findings:
        ${{ join(steps.research_topic.outputs.key_points, '\n') }}
        
        Requirements:
        - Target length: ${{ inputs.word_count }} words
        - Tone: ${{ inputs.tone }}
        - Audience: ${{ inputs.target_audience }}
        ${{ inputs.include_seo ? '- Include keywords: ' + join(inputs.keywords, ', ') : '' }}
        
        Return a structured outline with main sections and key points for each.
      outputs:
        outline:
          type: object
        estimated_sections:
          type: integer
    
    # Step 3: Write first draft
    - id: write_draft
      agent: writer
      prompt: |
        Write a ${{ inputs.content_type }} following this outline:
        ${{ toJSON(steps.create_outline.outputs.outline) }}
        
        Research to incorporate:
        - Key facts: ${{ join(steps.research_topic.outputs.key_points, '; ') }}
        - Statistics: ${{ join(steps.research_topic.outputs.statistics, '; ') }}
        
        Requirements:
        - Length: ${{ inputs.word_count }} words (±10%)
        - Tone: ${{ inputs.tone }}
        - Include citations for facts
        ${{ inputs.include_seo ? '- Naturally include keywords: ' + join(inputs.keywords, ', ') : '' }}
        
        Check grammar and style while writing.
      outputs:
        content:
          type: string
        word_count:
          type: integer
      updates:
        draft_version: 1
    
    # Step 4: Review and score the draft
    - id: review_draft
      agent: editor
      prompt: |
        Review this ${{ inputs.content_type }} draft:
        
        ${{ steps.write_draft.outputs.content }}
        
        Evaluate:
        1. Accuracy of information (check against sources)
        2. Clarity and readability
        3. Engagement and flow
        4. Appropriateness for ${{ inputs.target_audience }}
        5. Achievement of ${{ inputs.tone }} tone
        ${{ inputs.include_seo ? '6. SEO optimization and keyword usage' : '' }}
        
        Provide:
        - quality_score: 0-100
        - strengths: array of strong points
        - improvements: array of needed improvements
        - factual_errors: array of any errors found
      outputs:
        quality_score:
          type: integer
        strengths:
          type: array
        improvements:
          type: array
        factual_errors:
          type: array
      updates:
        quality_score: ${{ steps.review_draft.outputs.quality_score }}
    
    # Step 5: Revise if quality is below threshold
    - id: revise_content
      condition: ${{ steps.review_draft.outputs.quality_score < 80 }}
      agent: writer
      prompt: |
        Revise the content to address these issues:
        
        Improvements needed:
        ${{ join(steps.review_draft.outputs.improvements, '\n') }}
        
        Factual errors to fix:
        ${{ join(steps.review_draft.outputs.factual_errors, '\n') }}
        
        Original content:
        ${{ steps.write_draft.outputs.content }}
        
        Maintain the strengths identified:
        ${{ join(steps.review_draft.outputs.strengths, '\n') }}
        
        Keep the same requirements for length, tone, and audience.
      outputs:
        revised_content:
          type: string
        changes_made:
          type: array
      updates:
        draft_version: 2
        revisions_made: ${{ steps.revise_content.outputs.changes_made }}
    
    # Step 6: Final review of revised content
    - id: final_review
      condition: ${{ steps.review_draft.outputs.quality_score < 80 }}
      agent: editor
      prompt: |
        Final review of revised content:
        
        ${{ steps.revise_content.outputs.revised_content }}
        
        Changes made:
        ${{ join(steps.revise_content.outputs.changes_made, '\n') }}
        
        Confirm all issues have been addressed and provide final quality score.
      outputs:
        final_score:
          type: integer
        ready_to_publish:
          type: boolean
    
    # Step 7: Format final content
    - id: format_content
      agent: writer
      prompt: |
        Format the final content for ${{ inputs.content_type }}:
        
        Content: ${{ steps.review_draft.outputs.quality_score >= 80 ? steps.write_draft.outputs.content : steps.revise_content.outputs.revised_content }}
        
        Add:
        1. Appropriate headings and structure
        2. Meta description (if blog/article)
        3. Social media snippets (if requested)
        4. Source citations in proper format
        5. Call-to-action (if appropriate)
        
        Format as JSON with sections for easy parsing.
      outputs:
        formatted_content:
          type: object
        meta_description:
          type: string
        social_snippets:
          type: object
      updates:
        final_content: ${{ steps.format_content.outputs.formatted_content }}
  
  # Workflow outputs
  outputs:
    # Main content output
    content:
      body: ${{ state.final_content }}
      word_count: ${{ steps.write_draft.outputs.word_count }}
      meta_description: ${{ steps.format_content.outputs.meta_description }}
      social_snippets: ${{ steps.format_content.outputs.social_snippets }}
    
    # Quality metrics
    quality:
      initial_score: ${{ steps.review_draft.outputs.quality_score }}
      final_score: ${{ steps.final_review.outputs.final_score || steps.review_draft.outputs.quality_score }}
      revisions: ${{ state.draft_version }}
      improvements_made: ${{ state.revisions_made }}
    
    # Research data
    research:
      sources: ${{ state.sources }}
      key_findings: ${{ steps.research_topic.outputs.key_points }}
      statistics_used: ${{ steps.research_topic.outputs.statistics }}
    
    # Process metadata
    metadata:
      topic: ${{ inputs.topic }}
      type: ${{ inputs.content_type }}
      audience: ${{ inputs.target_audience }}
      tone: ${{ inputs.tone }}
      seo_optimized: ${{ inputs.include_seo }}
      keywords_used: ${{ inputs.keywords }}
```

## Data processor

```yaml
# Data Processing Workflow Example
# This workflow demonstrates intermediate Lacquer features including:
# - Multiple specialized agents
# - Conditional execution based on outputs
# - State management for tracking progress
# - Structured output parsing

version: "1.0"
metadata:
  name: data-processor
  description: |
    Processes and analyzes data with validation, transformation,
    and quality checks. Shows best practices for multi-step workflows.
  author: examples@lacquer.ai
  tags:
    - data-processing
    - validation
    - analytics
  version: 1.0.0

# Define specialized agents for different tasks
agents:
  # Validator agent with low temperature for consistent results
  validator:
    provider: openai
    model: gpt-4
    temperature: 0.1
    system_prompt: |
      You are a data validation expert. You:
      - Check data structure and format
      - Identify missing or invalid values
      - Ensure data meets quality standards
      - Return structured validation results
  
  # Analyzer agent for data insights
  analyzer:
    provider: openai
    model: gpt-4
    temperature: 0.3
    system_prompt: |
      You are a data analyst who:
      - Identifies patterns and trends
      - Calculates key statistics
      - Provides actionable insights
      - Returns analysis in JSON format
  
  # Transformer agent for data manipulation
  transformer:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.2
    system_prompt: |
      You are a data transformation specialist who:
      - Cleans and normalizes data
      - Applies requested transformations
      - Maintains data integrity
      - Documents all changes made

# Define workflow inputs
inputs:
  data:
    type: string
    description: The raw data to process (CSV, JSON, or plain text)
    required: true
  
  format:
    type: string
    description: Expected data format
    default: "auto"
  
  validation_rules:
    type: object
    description: Custom validation rules to apply
    default:
      require_headers: true
      allow_nulls: false
      min_rows: 1
  
  output_format:
    type: string
    description: Desired output format (json, csv, report)
    default: "json"

# Main workflow logic
workflow:
  # Initialize state for tracking progress
  state:
    processing_status: "started"
    validation_passed: false
    records_processed: 0
    errors: []
    warnings: []
  
  steps:
    # Step 1: Validate the input data
    - id: validate_data
      agent: validator
      prompt: |
        Validate this data according to the rules:
        
        Data: ${{ inputs.data }}
        Format: ${{ inputs.format }}
        Rules: ${{ toJSON(inputs.validation_rules) }}
        
        Return a JSON response with:
        - is_valid: boolean
        - format_detected: string
        - row_count: integer
        - column_count: integer (if applicable)
        - errors: array of error messages
        - warnings: array of warning messages
      outputs:
        is_valid:
          type: boolean
        format_detected:
          type: string
        row_count:
          type: integer
        column_count:
          type: integer
        errors:
          type: array
        warnings:
          type: array
      updates:
        validation_passed: ${{ steps.validate_data.outputs.is_valid }}
        errors: ${{ steps.validate_data.outputs.errors }}
        warnings: ${{ steps.validate_data.outputs.warnings }}
    
    # Step 2: Stop if validation failed
    - id: report_validation_failure
      condition: ${{ !steps.validate_data.outputs.is_valid }}
      agent: validator
      prompt: |
        Generate a validation failure report:
        - Errors found: ${{ join(steps.validate_data.outputs.errors, '; ') }}
        - Warnings: ${{ join(steps.validate_data.outputs.warnings, '; ') }}
        - Detected format: ${{ steps.validate_data.outputs.format_detected }}
        
        Provide recommendations for fixing the issues.
      updates:
        processing_status: "failed_validation"
    
    # Step 3: Analyze valid data
    - id: analyze_data
      condition: ${{ steps.validate_data.outputs.is_valid }}
      agent: analyzer
      prompt: |
        Analyze this validated data:
        
        Data: ${{ inputs.data }}
        Format: ${{ steps.validate_data.outputs.format_detected }}
        Rows: ${{ steps.validate_data.outputs.row_count }}
        Columns: ${{ steps.validate_data.outputs.column_count }}
        
        Provide analysis including:
        - summary_statistics: key metrics
        - patterns_found: identified patterns
        - anomalies: unusual data points
        - quality_score: 0-100
        - insights: actionable findings
      outputs:
        summary_statistics:
          type: object
        patterns_found:
          type: array
        anomalies:
          type: array
        quality_score:
          type: integer
        insights:
          type: array
    
    # Step 4: Transform data if quality is acceptable
    - id: transform_data
      condition: ${{ steps.analyze_data.outputs.quality_score >= 70 }}
      agent: transformer
      prompt: |
        Transform the data for output format: ${{ inputs.output_format }}
        
        Original data: ${{ inputs.data }}
        Analysis insights: ${{ join(steps.analyze_data.outputs.insights, '; ') }}
        
        Apply these transformations:
        1. Clean any identified anomalies
        2. Normalize formats
        3. Structure for ${{ inputs.output_format }} output
        
        Return the transformed data and a list of changes made.
      outputs:
        transformed_data:
          type: string
        changes_made:
          type: array
      updates:
        records_processed: ${{ steps.validate_data.outputs.row_count }}
        processing_status: "completed"
    
    # Step 5: Handle low quality data
    - id: handle_low_quality
      condition: ${{ steps.validate_data.outputs.is_valid && steps.analyze_data.outputs.quality_score < 70 }}
      agent: analyzer
      prompt: |
        The data quality score is low: ${{ steps.analyze_data.outputs.quality_score }}/100
        
        Issues found:
        - Anomalies: ${{ length(steps.analyze_data.outputs.anomalies) }}
        - Warnings: ${{ length(state.warnings) }}
        
        Recommend whether to:
        1. Proceed with warnings
        2. Request manual review
        3. Attempt automatic fixes
      outputs:
        recommendation:
          type: string
      updates:
        processing_status: "low_quality_warning"
  
  # Define workflow outputs
  outputs:
    # Status information
    status: ${{ state.processing_status }}
    success: ${{ state.processing_status == 'completed' }}
    
    # Validation results
    validation:
      passed: ${{ state.validation_passed }}
      format: ${{ steps.validate_data.outputs.format_detected }}
      errors: ${{ state.errors }}
      warnings: ${{ state.warnings }}
    
    # Processing results (conditional)
    results: ${{ state.processing_status == 'completed' ? {
      'transformed_data': steps.transform_data.outputs.transformed_data,
      'records_processed': state.records_processed,
      'quality_score': steps.analyze_data.outputs.quality_score,
      'insights': steps.analyze_data.outputs.insights,
      'changes': steps.transform_data.outputs.changes_made
    } : null }}
    
    # Analysis (if performed)
    analysis: ${{ steps.analyze_data.outputs || null }}
```
