# bash completion for local-docs-mcp (generated via go-flags)
_local_docs_mcp() {
    local args=("${COMP_WORDS[@]:1:$COMP_CWORD}")
    local IFS=$'\n'
    COMPREPLY=($(GO_FLAGS_COMPLETION=1 "${COMP_WORDS[0]}" "${args[@]}" 2>/dev/null))
    return 0
}
complete -o default -F _local_docs_mcp local-docs-mcp
