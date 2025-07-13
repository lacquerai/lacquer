#!/usr/bin/env node

const fs = require('fs');

function main() {
    // Get input from environment variable
    const inputsJson = process.env.LACQUER_INPUTS || '{}';
    const inputs = JSON.parse(inputsJson);
    
    const text = inputs.text || '';
    const operation = inputs.operation || 'analyze';
    
    const result = {
        runtime: 'node',
        version: process.version,
        timestamp: new Date().toISOString(),
        input_length: text.length
    };
    
    switch (operation) {
        case 'analyze':
            result.analysis = {
                word_count: text.split(/\s+/).filter(word => word.length > 0).length,
                character_count: text.length,
                line_count: text.split('\n').length,
                unique_words: [...new Set(text.toLowerCase().split(/\s+/))].length
            };
            break;
        case 'transform':
            result.transformed_text = text.toUpperCase();
            break;
        case 'extract':
            const words = text.split(/\s+/).filter(word => word.length > 5);
            result.long_words = words;
            break;
        default:
            result.error = `Unknown operation: ${operation}`;
    }
    
    console.log(JSON.stringify({ outputs: result }));
}

main();