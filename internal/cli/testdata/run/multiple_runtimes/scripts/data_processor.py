#!/usr/bin/env python3
import json
import os
import sys
from datetime import datetime

def main():
    # Get input from environment variable
    inputs_json = os.environ.get('LACQUER_INPUTS', '{}')
    inputs = json.loads(inputs_json)
    
    data = inputs.get('data', [])
    operation = inputs.get('operation', 'analyze')
    
    result = {
        'runtime': 'python',
        'version': sys.version.split()[0],
        'timestamp': datetime.now().isoformat(),
        'input_size': len(data)
    }
    
    if operation == 'analyze':
        if data:
            result['analysis'] = {
                'sum': sum(data),
                'mean': sum(data) / len(data),
                'min': min(data),
                'max': max(data),
                'count': len(data)
            }
        else:
            result['analysis'] = {'error': 'No data provided'}
    elif operation == 'transform':
        result['transformed_data'] = [x * 2 for x in data]
    elif operation == 'filter':
        result['filtered_data'] = [x for x in data if x > 10]
    
    print(json.dumps({'outputs': result}))

if __name__ == '__main__':
    main()