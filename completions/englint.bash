_englint_complete() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "help scan init version" -- "$cur") )
    return 0
  fi

  if [[ "${COMP_WORDS[1]}" == "scan" ]]; then
    case "$prev" in
      --config|--include|--exclude|--severity)
        return 0
        ;;
    esac
    COMPREPLY=( $(compgen -W "--config --exclude --include --json --fix --severity --no-color --verbose" -- "$cur") )
    return 0
  fi

  if [[ "${COMP_WORDS[1]}" == "init" ]]; then
    COMPREPLY=( $(compgen -W "--config" -- "$cur") )
    return 0
  fi
}

complete -F _englint_complete englint
