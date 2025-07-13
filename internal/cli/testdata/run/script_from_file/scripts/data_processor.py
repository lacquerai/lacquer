#!/usr/bin/env python3
import json
import sys
import os

def main():
    # Get input from environment variable (Lacquer passes inputs this way)
    inputs_json = os.environ.get('LACQUER_INPUTS', '{}')
    inputs = json.loads(inputs_json)
    
    # Extract parameters
    data = inputs.get('data', [])
    operation = inputs.get('operation', 'sum')
    
    result = {}
    
    if operation == 'sum':
        result['value'] = sum(data)
        result['operation'] = 'sum'
    elif operation == 'average':
        result['value'] = sum(data) / len(data) if data else 0
        result['operation'] = 'average'
    elif operation == 'max':
        result['value'] = max(data) if data else 0
        result['operation'] = 'max'
    elif operation == 'min':
        result['value'] = min(data) if data else 0
        result['operation'] = 'min'
    else:
        result['error'] = f'Unknown operation: {operation}'
        result['operation'] = operation
    
    result['input_count'] = len(data)
    result['input_data'] = data
    
    # Output result as JSON
    print(json.dumps({'outputs': result}))

if __name__ == '__main__':
    main()