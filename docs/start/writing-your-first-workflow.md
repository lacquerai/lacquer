# Writing Your First Workflow

This guide will walk you through creating your first Lacquer workflow by building a practical blog post writing system. We'll start simple and gradually add more sophisticated features, showing you why each concept matters along the way.

Let's begin by creating a new Lacquer workflow. The `laq init` command will guide you through setting up your first project.

If you haven't already, install Lacquer by following the [installation](start/installation.md) guide.

### Initialize Your Project

```bash
laq init
```

The interactive setup will ask you about your project:

```
? What's your project name? blog-writer
? Describe your workflow: A blog writing workflow that researches topics and creates engaging content
? Which model providers would you like to use? anthropic
? What script language do you prefer? go
```

This creates a `workflow.laq.yaml` file with a basic structure. Let's examine what was generated and understand each part:

```yaml
version: "1.0"
metadata:
  name: blog-writer
  description: A blog writing workflow that researches topics and creates engaging content

agents:
  writer:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: You are a helpful writing assistant.

inputs:
  topic:
    type: string
    description: The topic to write about
    required: true

workflow:
  steps:
    - id: write_post
      agent: writer
      prompt: Write a blog post about ${{ inputs.topic }}
  
  outputs:
    blog_post: ${{ steps.write_post.output }}
```

This basic workflow takes a topic as input and generates a blog post. But real-world content creation needs more sophistication. Let's build this up step by step.

## Step 1: Adding a Research Agent

Good blog posts start with solid research. Let's add a dedicated research agent that's optimized for gathering information. This demonstrates why you might want multiple agents with different capabilities.

```yaml
agents:
  # Our existing writer
  writer:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: |
      You are an expert content writer who creates engaging, well-structured blog posts.
      Focus on clear explanations, compelling storytelling, and actionable insights.
  
  # New specialized research agent
  researcher:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.3  # Lower temperature for more factual, focused research
    system_prompt: |
      You are a meticulous researcher who finds comprehensive, accurate information.
      Always provide specific details, recent examples, and credible insights.
      Structure your research with clear key points and supporting evidence.
```

**Why separate agents?** Different tasks benefit from different configurations. Our researcher uses a lower temperature (0.3) for more factual, consistent results, while our writer uses higher temperature (0.7) for creativity. Their system prompts are tailored to their specific roles.

You can find more information about agents in the [agents](concepts/agents.md) concept guide.

## Step 2: Adding a Research Step

Now let's add a research step that feeds into our writing process. This shows how steps can reference each other's outputs:

```yaml
workflow:
  steps:
    # First, research the topic thoroughly
    - id: research_topic
      agent: researcher
      prompt: |
        Research the topic: "${{ inputs.topic }}"

        Provide:

        1. Key facts and recent developments
        2. Common questions people have about this topic
        3. Practical examples or case studies
        4. Current trends and future outlook
        Structure your findings clearly for a content writer to use.

      outputs:
        key_facts:
          type: array
          description: Key facts about the topic
        questions:
          type: array  
          description: Common questions people ask
        examples:
          type: array
          description: Practical examples or case studies
    
    # Then write the blog post using the research
    - id: write_post
      agent: writer
      prompt: |
        Write an engaging blog post about "${{ inputs.topic }}" using this research:
        
        Key Facts:
        ${{ join(steps.research_topic.outputs.key_facts, '\n- ') }}
        
        Common Questions:
        ${{ join(steps.research_topic.outputs.questions, '\n- ') }}
        
        Examples:
        ${{ join(steps.research_topic.outputs.examples, '\n- ') }}
        
        Create a well-structured, engaging blog post that addresses these questions
        and incorporates the key facts and examples naturally.
```

**Why reference previous steps?** This creates a pipeline where each step builds on the previous one. The writer now has structured research data instead of starting from scratch, leading to more informed and comprehensive content.

You can find more information about steps in the [workflow structure](concepts/workflow-structure.md) concept guide and variables in the [variables](concepts/variables.md) concept guide.

## Step 3: Enhancing Workflow Outputs

Let's improve our outputs to provide more useful, structured results. This makes our workflow more valuable for real-world use:

```yaml
workflow:
  steps:
    - id: research_topic
      agent: researcher
      prompt: |
        Research the topic: "${{ inputs.topic }}"
        
        Provide comprehensive research structured as follows:
        1. Key facts and recent developments
        2. Common questions people have about this topic  
        3. Practical examples or case studies
        4. Current trends and future outlook
      outputs:
        key_facts:
          type: array
          description: Key facts about the topic
        questions:
          type: array
          description: Common questions people ask
        examples:
          type: array
          description: Practical examples or case studies
        trends:
          type: array
          description: Current trends and future outlook
    
    - id: write_post
      agent: writer
      prompt: |
        Write an engaging blog post about "${{ inputs.topic }}" using this research:
        
        Key Facts:
        ${{ join(steps.research_topic.outputs.key_facts, '\n- ') }}
        
        Common Questions:
        ${{ join(steps.research_topic.outputs.questions, '\n- ') }}
        
        Examples:
        ${{ join(steps.research_topic.outputs.examples, '\n- ') }}
        
        Trends:
        ${{ join(steps.research_topic.outputs.trends, '\n- ') }}
        
        Structure the post with:
        1. Compelling headline
        2. Engaging introduction
        3. Well-organized main content
        4. Clear conclusion with key takeaways
      outputs:
        headline:
          type: string
          description: The blog post headline
        content:
          type: string
          description: The full blog post content
        word_count:
          type: integer
          description: Number of words in the post
        key_takeaways:
          type: array
          description: Main takeaways from the post
  
  # More detailed outputs
  outputs:
    headline: ${{ steps.write_post.outputs.headline }}
    blog_post: ${{ steps.write_post.outputs.content }}
    word_count: ${{ steps.write_post.outputs.word_count }}
    research_summary:
      key_facts: ${{ steps.research_topic.outputs.key_facts }}
      questions_addressed: ${{ steps.research_topic.outputs.questions }}
    takeaways: ${{ steps.write_post.outputs.key_takeaways }}
```

**Why structure outputs?** Structured outputs make your workflow results more useful. Instead of just getting raw text, you get organized data you can use in other systems, reports, or workflows.

You can find more information about outputs in the [workflow structure](concepts/workflow-structure.md) concept guide.

## Step 4: Adding Conditional Logic

Let's add a quality review step that only triggers if the content is too short. This demonstrates conditional execution:

```yaml
inputs:
  topic:
    type: string
    description: The topic to write about
    required: true
  min_words:
    type: integer
    description: Minimum word count for the blog post
    default: 800

workflow:
  steps:
    - id: research_topic
      agent: researcher
      prompt: |
        Research the topic: "${{ inputs.topic }}"
        Provide comprehensive research with key facts, common questions, 
        practical examples, and current trends.
      outputs:
        key_facts:
          type: array
        questions:
          type: array
        examples:
          type: array
        trends:
          type: array
    
    - id: write_post
      agent: writer
      prompt: |
        Write an engaging blog post about "${{ inputs.topic }}" using this research.
        Target length: at least ${{ inputs.min_words }} words.
        
        Research findings:
        Key Facts: ${{ join(steps.research_topic.outputs.key_facts, '\n- ') }}
        Questions: ${{ join(steps.research_topic.outputs.questions, '\n- ') }}
        Examples: ${{ join(steps.research_topic.outputs.examples, '\n- ') }}
        Trends: ${{ join(steps.research_topic.outputs.trends, '\n- ') }}
      outputs:
        headline:
          type: string
        content:
          type: string
        word_count:
          type: integer
        key_takeaways:
          type: array
    
    # Conditional step: only expand if content is too short
    - id: expand_content
      condition: ${{ steps.write_post.outputs.word_count < inputs.min_words }}
      agent: writer
      prompt: |
        The current blog post is only ${{ steps.write_post.outputs.word_count }} words, 
        but we need at least ${{ inputs.min_words }} words.
        
        Please expand the following content by adding more detail, examples, 
        and explanations while maintaining quality:
        
        ${{ steps.write_post.outputs.content }}
        
        Focus on adding value, not just filler content.
      outputs:
        expanded_content:
          type: string
        final_word_count:
          type: integer
  
  outputs:
    headline: ${{ steps.write_post.outputs.headline }}
    # Use expanded content if available, otherwise use original
    blog_post: ${{ steps.expand_content.outputs.expanded_content || steps.write_post.outputs.content }}
    word_count: ${{ steps.expand_content.outputs.final_word_count || steps.write_post.outputs.word_count }}
    research_summary:
      key_facts: ${{ steps.research_topic.outputs.key_facts }}
      questions_addressed: ${{ steps.research_topic.outputs.questions }}
    takeaways: ${{ steps.write_post.outputs.key_takeaways }}
    quality_enhanced: ${{ steps.expand_content.outputs.expanded_content != null }}
```

**Why use conditionals?** Conditional logic makes your workflows adaptive and efficient. The expansion step only runs when needed, saving time and API costs. This pattern is useful for quality gates, error handling, and feature toggles based on user preferences or data characteristics.

You can find more information about conditionals in the [control flow](concepts/control-flow.md) concept guide.

## Step 5: Adding a Script Step

Finally, let's add a script step that analyzes our content for SEO optimization. This shows how to integrate custom logic alongside AI agents:

First, let's create a simple SEO analysis script. Create a `scripts/` directory and add `seo_analyzer.go`:

```go
// scripts/seo_analyzer.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Input struct {
	Headline string `json:"headline"`
	Content  string `json:"content"`
	Topic    string `json:"topic"`
}

type InputWrapper struct {
    Inputs Input `json:"inputs"`
}

type SEOAnalysis struct {
	HeadlineLength    int      `json:"headline_length"`
	KeywordDensity    float64  `json:"keyword_density"`
	ReadabilityScore  int      `json:"readability_score"`
	Recommendations   []string `json:"recommendations"`
	SEOScore          int      `json:"seo_score"`
}

func main() {
	var input InputWrapper
	json.NewDecoder(os.Stdin).Decode(&input)
	
	analysis := analyzeSEO(input.Inputs)
	json.NewEncoder(os.Stdout).Encode(analysis)
}

func analyzeSEO(input Input) SEOAnalysis {
    // custom seo analysis logic here
	
	return analysis
}
```

Now update your workflow to include the SEO analysis:

```yaml

# install the script requirements into the pipeline if they are not already in the system
requirements:
  runtimes:
    - name: go
      version: "1.24"

workflow:
  steps:
    - id: research_topic
      agent: researcher
      prompt: |
        Research the topic: "${{ inputs.topic }}"
        Provide comprehensive research with key facts, common questions, 
        practical examples, and current trends.
      outputs:
        key_facts:
          type: array
        questions:
          type: array
        examples:
          type: array
        trends:
          type: array
    
    - id: write_post
      agent: writer
      prompt: |
        Write an engaging blog post about "${{ inputs.topic }}" using this research.
        Target length: at least ${{ inputs.min_words }} words.
        
        Research findings:
        Key Facts: ${{ join(steps.research_topic.outputs.key_facts, '\n- ') }}
        Questions: ${{ join(steps.research_topic.outputs.questions, '\n- ') }}
        Examples: ${{ join(steps.research_topic.outputs.examples, '\n- ') }}
        Trends: ${{ join(steps.research_topic.outputs.trends, '\n- ') }}
      outputs:
        headline:
          type: string
        content:
          type: string
        word_count:
          type: integer
        key_takeaways:
          type: array
    
    - id: expand_content
      condition: ${{ steps.write_post.outputs.word_count < inputs.min_words }}
      agent: writer
      prompt: |
        The current blog post is only ${{ steps.write_post.outputs.word_count }} words, 
        but we need at least ${{ inputs.min_words }} words.
        
        Please expand the following content by adding more detail, examples, 
        and explanations while maintaining quality:
        
        ${{ steps.write_post.outputs.content }}
      outputs:
        expanded_content:
          type: string
        final_word_count:
          type: integer
    
    # Script step: Analyze SEO metrics
    - id: seo_analysis
      run: "go run scripts/seo_analyzer.go"
      with:
        headline: ${{ steps.write_post.outputs.headline }}
        content: ${{ steps.expand_content.outputs.expanded_content || steps.write_post.outputs.content }}
        topic: ${{ inputs.topic }}
    
    # Optional: Improve content based on SEO analysis
    - id: seo_optimization
      condition: ${{ fromJSON(steps.seo_analysis.output).seo_score < 70 }}
      agent: writer
      prompt: |
        Please optimize this blog post for SEO based on the following analysis:
        
        Current SEO Score: ${{ fromJSON(steps.seo_analysis.output).seo_score }}/100
        
        Recommendations:
        ${{ join(fromJSON(steps.seo_analysis.output).recommendations, '\n- ') }}
        
        Original content:
        Headline: ${{ steps.write_post.outputs.headline }}
        Content: ${{ steps.expand_content.outputs.expanded_content || steps.write_post.outputs.content }}
        
        Please revise the headline and content to improve SEO while maintaining quality and readability.
      outputs:
        optimized_headline:
          type: string
        optimized_content:
          type: string
  
  outputs:
    headline: ${{ steps.seo_optimization.outputs.optimized_headline || steps.write_post.outputs.headline }}
    blog_post: ${{ steps.seo_optimization.outputs.optimized_content || steps.expand_content.outputs.expanded_content || steps.write_post.outputs.content }}
    word_count: ${{ steps.expand_content.outputs.final_word_count || steps.write_post.outputs.word_count }}
    seo_analysis: ${{ fromJSON(steps.seo_analysis.output) }}
    research_summary:
      key_facts: ${{ steps.research_topic.outputs.key_facts }}
      questions_addressed: ${{ steps.research_topic.outputs.questions }}
    takeaways: ${{ steps.write_post.outputs.key_takeaways }}
    quality_enhanced: ${{ steps.expand_content.outputs.expanded_content != null }}
    seo_optimized: ${{ steps.seo_optimization.outputs.optimized_content != null }}
```

**Why use script steps?** Script steps let you integrate custom logic, data processing, or external tools that AI models can't handle directly. In this case, we're doing quantitative SEO analysis that requires specific calculations. Scripts can also interact with databases, APIs, or file systems.

## Step 6: Adding Agent Tools

So far our research agent has been working from its training data, but what if we want it to access real-time information? Let's enhance our research agent by giving it a web search tool. This demonstrates how tools can extend agent capabilities beyond their base models.

First, let's create a simple web search tool script. Create `scripts/web_search.go`:

```go
// scripts/web_search.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

type SearchRequest struct {
	Query string `json:"query"`
}

type InputWrapper struct {
	Inputs SearchRequest `json:"inputs"`
}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}


type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
}

func main() {
	var req InputWrapper
	json.NewDecoder(os.Stdin).Decode(&req)
	
	results := performSearch(req.Inputs.Query)
	json.NewEncoder(os.Stdout).Encode(results)
}

func performSearch(query string) SearchResponse {
	// For this example, we'll simulate search results
	// In a real implementation, you'd integrate with a search API like Google Custom Search, Bing, or DuckDuckGo
	
	simulatedResults := []SearchResult{
		{
			Title:   fmt.Sprintf("Latest developments in %s", query),
			URL:     "https://example.com/article1",
			Snippet: fmt.Sprintf("Recent research shows significant progress in %s with new breakthroughs emerging in 2024...", query),
        },
	}
	
	return SearchResponse{
		Results: simulatedResults,
		Query:   query,
	}
}
```

Now let's update our workflow to give the research agent access to this search tool:

```yaml
agents:
  # Enhanced research agent with web search capability
  researcher:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.3
    system_prompt: |
      You are a meticulous researcher who finds comprehensive, accurate information.
      You have access to a web search tool to find current, real-time information.
      Always combine your knowledge with fresh search results for the most up-to-date research.
      Structure your research with clear key points and supporting evidence.
    tools:
      - name: web_search
        description: Search the web for current information about any topic
        script: "go run scripts/web_search.go"
        parameters:
          type: object
          properties:
            query:
              type: string
              description: The search query to execute
          required:
            - query
  
  # Writer agent remains the same
  writer:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    temperature: 0.7
    system_prompt: |
      You are an expert content writer who creates engaging, well-structured blog posts.
      Focus on clear explanations, compelling storytelling, and actionable insights.
```

Now update the research step to use the search tool:

```yaml
workflow:
  steps:
    - id: research_topic
      agent: researcher
      prompt: |
        Research the topic: "${{ inputs.topic }}"
        
        You can use the web_search tool to find current information, then combine it with your knowledge to provide:

        1. Key facts and recent developments
        2. Common questions people have about this topic  
        3. Practical examples or case studies
        4. Current trends and future outlook
        
        Start by searching for recent information about this topic.
      outputs:
        key_facts:
          type: array
        questions:
          type: array
        examples:
          type: array
        trends:
          type: array
        sources:
          type: array
          description: Web sources found during research
```

The research agent can now call the web search tool during its research process. The agent will automatically decide when to use the tool based on the prompt and will integrate the search results into its response.

**Why use tools with agents?** Tools extend agents beyond their training data cutoff, enabling real-time information access, database queries, API calls, file system operations, and more. This makes agents much more powerful and useful for real-world applications where current information matters.

**Tool Integration Benefits:**
- **Real-time data**: Access current information beyond training cutoffs
- **External systems**: Integrate with databases, APIs, and services
- **Custom capabilities**: Add domain-specific functionality
- **Accuracy**: Verify information with authoritative sources
- **Automation**: Let agents autonomously gather needed information

Lacquer supports local tools and MCP servers. You can find more information about tools in the [tools](concepts/tools.md) concept.

## Running Your Workflow

Now you can run your sophisticated blog writing workflow:

```bash
laq run --topic "The Future of AI in Healthcare" --min-words 1000
```

This will:
1. Research the topic comprehensively
2. Write an initial blog post using the research
3. Expand the content if it's too short
4. Analyze SEO metrics using your custom script
5. Optimize for SEO if the score is too low
6. Return structured results with metadata

## What You've Built

Congratulations! You've created a production-ready blog writing workflow that demonstrates all the key Lacquer concepts:

- **Multiple specialized agents** for different tasks
- **Sequential steps** that build on each other's outputs
- **Structured outputs** for organized results
- **Conditional logic** for adaptive behavior
- **Script integration** for custom processing
- **Complex variable interpolation** and data flow

## Learn More

Now that you understand the basics, explore these concepts in detail:

- **[Workflow Structure](concepts/workflow-structure.md)** - Learn about the complete workflow file format and all available options
- **[Agents](concepts/agents.md)** - Dive deeper into agent configuration, different providers, and advanced features like tool integration
- **[Workflow Steps](concepts/workflow-steps.md)** - Explore all step types including container steps, child workflows, and advanced execution patterns
- **[Control Flow](concepts/control-flow.md)** - Master conditional execution, loops, and complex workflow logic
- **[Variable Interpolation](concepts/variables.md)** - Learn advanced expression syntax, functions, and dynamic value generation
- **[State Management](concepts/state-management.md)** - Build stateful workflows that maintain data across steps and iterations
- **[Tools](concepts/tools.md)** - Extend your agents with custom tools, MCP servers, and external integrations