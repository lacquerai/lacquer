
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

window.$docsify = {
    name: 'Lacquer',
    logo: '/_media/logo.png',
    auto2top: true,
    loadSidebar: true,
    subMaxLevel: 2,
    markdown: {
        renderer: {
            code: function (code, lang) {
                if (lang === 'yaml') {
                    // Wrap YAML multiline strings in token-multiline spans before highlighting
                    const highlight = Prism.highlight(code, Prism.languages.yaml, 'yaml');
                    const processedCode = wrapYamlMultilineStrings(highlight);
                    return `<pre data-lang="yaml"><code class="lang-yaml">${processedCode}</code></pre>`;
                }

                if (!Prism.languages[lang]) {
                    return `<pre><code class="lang-${lang}">${code}</code></pre>`;
                }

                const highlight = Prism.highlight(code, Prism.languages[lang], lang);
                return `<pre data-lang="${lang}"><code class="lang-${lang}">${highlight}</code></pre>`;
            }
        }
    }
}