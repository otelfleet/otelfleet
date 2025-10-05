import {ReactFlowProvider} from "reactflow";
import {React, useState} from "react";
import MonacoEditor from "@monaco-editor/react";

export function Editor({locked, setLocked}){
    const [code, setCode] = useState()
    return (
        <>
            <ReactFlowProvider>
                <MonacoEditor
                    height="400px"
                    defaultLanguage="yaml"
                    defaultValue="code"
                    onChange={(value) => setCode(value)}
                ></MonacoEditor>                
            </ReactFlowProvider>
        </>
    )
}