#!/usr/bin/env node

function main() {
    let inputData = '';
    
    // Read from stdin
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => {
        inputData += chunk;
    });
    
    process.stdin.on('end', () => {
        let inputs = {};
        
        // Parse input JSON
        try {
            if (inputData) {
                inputs = JSON.parse(inputData);
            }
        } catch (e) {
            inputs = {};
        }
        
        // Extract any test parameter
        const testParam = inputs.inputs.test_param || 'default_value';
        
        const result = {
            runtime: 'node',
            version: process.version,
            message: `Received input: ${testParam}`,
        };
        
        console.log(JSON.stringify(result));
    });
}

main();