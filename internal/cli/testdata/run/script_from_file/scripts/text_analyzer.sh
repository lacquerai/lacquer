#!/bin/bash

# This script analyzes text and returns various metrics
text="$1"
analysis_type="$2"

if [ -z "$text" ]; then
    echo '{"error": "No text provided"}'
    exit 1
fi

case "$analysis_type" in
    "basic")
        word_count=$(echo "$text" | wc -w | tr -d ' ')
        char_count=${#text}
        line_count=$(echo "$text" | wc -l | tr -d ' ')
        
        echo "{
            \"word_count\": $word_count,
            \"character_count\": $char_count,
            \"line_count\": $line_count,
            \"analysis_type\": \"basic\",
            \"original_text\": \"$text\"
        }"
        ;;
    "detailed")
        word_count=$(echo "$text" | wc -w | tr -d ' ')
        char_count=${#text}
        line_count=$(echo "$text" | wc -l | tr -d ' ')
        unique_words=$(echo "$text" | tr ' ' '\n' | sort | uniq | wc -l | tr -d ' ')
        
        # Check if text contains numbers
        has_numbers="false"
        if echo "$text" | grep -q '[0-9]'; then
            has_numbers="true"
        fi
        
        # Check if text is all uppercase
        is_uppercase="false"
        if [ "$text" = "$(echo "$text" | tr '[:lower:]' '[:upper:]')" ]; then
            is_uppercase="true"
        fi
        
        echo "{
            \"word_count\": $word_count,
            \"character_count\": $char_count,
            \"line_count\": $line_count,
            \"unique_words\": $unique_words,
            \"has_numbers\": $has_numbers,
            \"is_uppercase\": $is_uppercase,
            \"analysis_type\": \"detailed\",
            \"original_text\": \"$text\"
        }"
        ;;
    *)
        echo "{\"error\": \"Unknown analysis type: $analysis_type\"}"
        exit 1
        ;;
esac