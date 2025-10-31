import Editor from '@monaco-editor/react'
import { useRef } from 'react'
import { SCRIPT_TYPE_DEFINITIONS } from '~/lib/script-types'

interface ScriptEditorProps {
  value: string
  onChange: (value: string) => void
  readOnly?: boolean
  height?: string
}

export function ScriptEditor({ value, onChange, readOnly = false, height = '400px' }: ScriptEditorProps) {
  const editorRef = useRef<any>(null)

  const handleEditorDidMount = (editor: any, monaco: any) => {
    editorRef.current = editor

    // Add BroMQ type definitions for intellisense
    monaco.languages.typescript.javascriptDefaults.addExtraLib(
      SCRIPT_TYPE_DEFINITIONS,
      'ts:filename/bromq-globals.d.ts'
    )

    // Configure JavaScript compiler options
    monaco.languages.typescript.javascriptDefaults.setCompilerOptions({
      target: monaco.languages.typescript.ScriptTarget.ES5,
      allowNonTsExtensions: true,
      moduleResolution: monaco.languages.typescript.ModuleResolutionKind.NodeJs,
      module: monaco.languages.typescript.ModuleKind.CommonJS,
      noEmit: true,
      typeRoots: ['node_modules/@types'],
    })

    // Configure diagnostics options
    monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
      noSemanticValidation: false,
      noSyntaxValidation: false,
    })
  }

  return (
    <div className="rounded-md border overflow-hidden">
      <Editor
        height={height}
        defaultLanguage="javascript"
        value={value}
        onChange={(val) => onChange(val || '')}
        onMount={handleEditorDidMount}
        theme="vs-dark"
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 14,
          tabSize: 2,
          readOnly,
          lineNumbers: 'on',
          renderLineHighlight: 'all',
          quickSuggestions: {
            other: true,
            comments: false,
            strings: true,
          },
          suggestOnTriggerCharacters: true,
          acceptSuggestionOnEnter: 'on',
          tabCompletion: 'on',
          wordBasedSuggestions: 'off',
          parameterHints: {
            enabled: true,
          },
        }}
      />
    </div>
  )
}
