import React, {createContext, useContext, useState} from "react";

type EditorContextValue = {
    id : string
    contents : string
};

const EditorContext = createContext<EditorContextValue | null>(null);

export function Editor(props: React.PropsWithChildren) {
    return (
        <EditorContext.Provider value={{contents:""}}>
            {props.children}
        </EditorContext.Provider>
    )
}

function SaveButton() {
    return (
        <div> Save </div>
    )
}

Editor.SaveButton = SaveButton