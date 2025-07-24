const fs = require('fs');
const path = require('path');
const Prism = require('prismjs');

// Load YAML language support for Prism
require('prismjs/components/prism-yaml');

/**
 * Reads all .yml files from the tabs directory and formats them with Prism
 * @param {string} tabsDir - Path to the tabs directory
 * @returns {Object} Object mapping filenames to formatted HTML
 */
function formatTabFiles(tabsDir = './tabs') {
  const formattedFiles = {};
  
  try {
    // Read all files in the tabs directory
    const files = fs.readdirSync(tabsDir);
    
    // Filter for .yml and .yaml files
    const yamlFiles = files.filter(file => 
      file.endsWith('.yml') || file.endsWith('.yaml')
    );
    
    console.log(`Found ${yamlFiles.length} YAML files in ${tabsDir}`);
    
    yamlFiles.forEach(filename => {
      const filepath = path.join(tabsDir, filename);
      const content = fs.readFileSync(filepath, 'utf8');
      
      // Use Prism to highlight the YAML content
      let highlighted = Prism.highlight(content, Prism.languages.yaml, 'yaml');
      highlighted = wrapYamlMultilineStrings(highlighted);
      
      // Generate tab-friendly name (remove .laq.yml extension)
      const tabName = filename.replace(/\.laq\.ya?ml$/, '');
      
      formattedFiles[tabName] = {
        filename,
        content,
        highlighted,
        htmlOutput: `<pre><code class="lang-yaml">
${highlighted}
</code></pre>`
      };
      
      console.log(`✓ Formatted ${filename} -> ${tabName}`);
    });
    
  } catch (error) {
    console.error('Error processing files:', error.message);
    return {};
  }
  
  return formattedFiles;
}

/**
 * Generates HTML tab content for the website
 * @param {Object} formattedFiles - Object from formatTabFiles()
 * @returns {string} HTML content for tabs
 */
function generateTabHTML(formattedFiles) {
  const tabTriggers = [];
  const tabContents = [];
  
  Object.entries(formattedFiles).forEach(([tabName, data], index) => {
    const isActive = index === 0 ? ' active' : '';
    
    // Generate tab trigger
    tabTriggers.push(
      `<button class="tab-trigger${isActive}" data-tab="${tabName}">${formatTabTitle(tabName)}</button>`
    );
    
    // Generate tab content
    tabContents.push(`
    <div class="tab-content${isActive}" id="tab-${tabName}">
      <div class="code-block">
        ${data.htmlOutput}
      </div>
    </div>`);
  });
  
  return {
    triggers: tabTriggers.join('\n        '),
    contents: tabContents.join('\n')
  };
}

/**
 * Formats tab name for display
 * @param {string} tabName - Raw tab name
 * @returns {string} Formatted title
 */
function formatTabTitle(tabName) {
  const titleMap = {
    'scripts': 'Script steps',
    'conditionals': 'Conditionals', 
    'state': 'State management',
    'tools': 'Tool use',
    'mcp': 'MCP support'
  };
  
  return titleMap[tabName] || tabName.replace(/[-_]/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
}

/**
 * Writes formatted output to files
 * @param {Object} formattedFiles - Formatted file data
 * @param {string} outputDir - Output directory
 */
function writeOutputFiles(formattedFiles, outputDir = './formatted') {
  // Create output directory if it doesn't exist
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }
  
  // Write individual formatted files
  Object.entries(formattedFiles).forEach(([tabName, data]) => {
    const outputPath = path.join(outputDir, `${tabName}.html`);
    fs.writeFileSync(outputPath, data.htmlOutput);
    console.log(`✓ Wrote ${outputPath}`);
  });
  
  // Write combined tab HTML
  const tabHTML = generateTabHTML(formattedFiles);
  const combinedPath = path.join(outputDir, 'tabs.html');
  const combinedContent = `<!-- Tab Triggers -->
<div class="tabs">
  <div class="tabs-list">
      ${tabHTML.triggers}
  </div>

  <!-- Tab Contents -->
  ${tabHTML.contents}
</div>
`;
  
  fs.writeFileSync(combinedPath, combinedContent);
  console.log(`✓ Wrote combined tabs to ${combinedPath}`);
  
  // Write JSON data file
  const jsonPath = path.join(outputDir, 'formatted-data.json');
  fs.writeFileSync(jsonPath, JSON.stringify(formattedFiles, null, 2));
  console.log(`✓ Wrote data to ${jsonPath}`);
}

// Main execution
function main() {
  console.log('🎨 Formatting YAML files with Prism...');
  
  const tabsDir = path.join(__dirname, 'tabs');
  const outputDir = path.join(__dirname, 'formatted');
  
  // Check if tabs directory exists
  if (!fs.existsSync(tabsDir)) {
    console.error(`❌ Tabs directory not found: ${tabsDir}`);
    process.exit(1);
  }
  
  // Format the files
  const formattedFiles = formatTabFiles(tabsDir);
  
  if (Object.keys(formattedFiles).length === 0) {
    console.error('❌ No YAML files found or processed');
    process.exit(1);
  }
  
  // Write output files
  writeOutputFiles(formattedFiles, outputDir);
  
  console.log('\n🎉 All files formatted successfully!');
  console.log(`📁 Output directory: ${outputDir}`);
  console.log('📄 Files generated:');
  console.log('   - Individual HTML files for each tab');
  console.log('   - tabs.html (combined tab HTML)');
  console.log('   - formatted-data.json (raw data)');
}

// Export functions for use as module
module.exports = {
  formatTabFiles,
  generateTabHTML,
  formatTabTitle,
  writeOutputFiles
};

function wrapYamlMultilineStrings(htmlCode) {
  try {
    const lines = htmlCode.split('\n');
    const result = [];
    let inMultiline = false;
    let multilineIndent = -1;
    let multilineBuffer = [];

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      // Calculate the actual indentation (ignoring HTML tags)
      const strippedLine = line.replace(/<[^>]+>/g, '');
      const currentIndent = strippedLine.search(/\S/);

      // Check if we need to close a multiline block
      if (inMultiline) {
        if (currentIndent !== -1 && currentIndent <= multilineIndent && !strippedLine.match(/^\s*$/)) {
          // End of multiline block - flush buffer with wrapper
          if (multilineBuffer.length > 0) {
            const firstLine = multilineBuffer[0];
            const remainingLines = multilineBuffer.slice(1);

            // Wrap the pipe character and everything after in the multiline span
            const wrappedFirst = firstLine.replace(
              /(<span class="token punctuation">\|<\/span>)/,
              '<span class="token-multiline">$1'
            );

            result.push(wrappedFirst);
            result.push(...remainingLines);
            result[result.length - 1] += '</span>';
          }

          // Reset multiline state
          inMultiline = false;
          multilineIndent = -1;
          multilineBuffer = [];

          // Process current line normally
          result.push(line);
        } else {
          // Continue collecting multiline content
          multilineBuffer.push(line);
        }
      } else {
        // Check if this line starts a multiline string
        if (line.includes(':</span>') && line.includes('<span class="token punctuation">|</span>')) {
          inMultiline = true;
          multilineIndent = currentIndent;
          multilineBuffer = [line];
        } else {
          result.push(line);
        }
      }
    }

    // Handle any remaining multiline content at end of file
    if (inMultiline && multilineBuffer.length > 0) {
      const firstLine = multilineBuffer[0];
      const remainingLines = multilineBuffer.slice(1);

      const wrappedFirst = firstLine.replace(
        /(<span class="token punctuation">\|<\/span>)/,
        '<span class="token-multiline">$1'
      );

      result.push(wrappedFirst);
      result.push(...remainingLines);
      result[result.length - 1] += '</span>';
    }

    return result.join('\n');
  } catch (e) {
    console.error(e);
    return htmlCode;
  }
}

// Run if called directly
if (require.main === module) {
  main();
}
