#compdef local-docs-mcp

# zsh completion for local-docs-mcp (generated via go-flags)
_local_docs_mcp() {
    local IFS=$'\n'
    local -a completions
    completions=($(GO_FLAGS_COMPLETION=1 "${words[1]}" "${(@)words[2,$CURRENT]}" 2>/dev/null))
    if (( ${#completions} )); then
        compadd -- "${completions[@]}"
    else
        _files
    fi
}

_local_docs_mcp "$@"
