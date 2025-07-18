#!/usr/bin/env python3
import json
import sys

def main():
    # Read input from stdin
    try:
        input_data = sys.stdin.read()
        if input_data:
            inputs = json.loads(input_data)
        else:
            inputs = {}
    except json.JSONDecodeError:
        inputs = {}
    
    # Extract any test parameter
    test_param = inputs.get('inputs', {}).get('test_param', 'default_value')
    
    result = {
        'runtime': 'python',
        'version': sys.version.split()[0],
        'message': f'Received input: {test_param}',
    }
    
    print(json.dumps({'outputs': result}))

if __name__ == '__main__':
    main()