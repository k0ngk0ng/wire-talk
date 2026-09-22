# bash completion for wirectl talk and wirectl-talk.
# Load with: source <(wirectl talk completion bash)

_wirectl_talk_completion() {
    local cur prev command_index command options plugin name
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "${COMP_WORDS[0]}" in
        wirectl)
            if (( COMP_CWORD == 1 )); then
                options="talk version"
                for plugin in $(compgen -c 2>/dev/null); do
                    case "$plugin" in
                        wirectl-*)
                            name=${plugin#wirectl-}
                            options="$options $name"
                            ;;
                    esac
                done
                COMPREPLY=( $(compgen -W "$options" -- "$cur") )
                return 0
            fi
            [[ "${COMP_WORDS[1]}" == "talk" ]] || return 0
            command_index=2
            ;;
        wirectl-talk)
            command_index=1
            ;;
        *)
            return 0
            ;;
    esac

    if (( COMP_CWORD == command_index )); then
        COMPREPLY=( $(compgen -W "completion daemon devices init invite join mute pair status unmute update version watch" -- "$cur") )
        return 0
    fi

    # --state-dir is accepted before or after every talk command.
    if [[ "$prev" == "--state-dir" ]]; then
        COMPREPLY=( $(compgen -d -- "$cur") )
        return 0
    fi

    command="${COMP_WORDS[command_index]}"
    case "$command" in
        completion)
            if (( COMP_CWORD == command_index + 1 )); then
                COMPREPLY=( $(compgen -W "bash zsh" -- "$cur") )
            fi
            ;;
        daemon)
            if (( COMP_CWORD == command_index + 1 )); then
                COMPREPLY=( $(compgen -W "install start status stop" -- "$cur") )
                return 0
            fi
            case "${COMP_WORDS[command_index+1]}" in
                status)
                    options="--json --state-dir --help"
                    ;;
                install|start|stop)
                    options="--state-dir --help"
                    ;;
                *)
                    options="--json --state-dir --help"
                    ;;
            esac
            COMPREPLY=( $(compgen -W "$options" -- "$cur") )
            ;;
        devices)
            COMPREPLY=( $(compgen -W "--json --state-dir --help" -- "$cur") )
            ;;
        init)
            case "$prev" in
                --key-file)
                    COMPREPLY=( $(compgen -f -- "$cur") )
                    return 0
                    ;;
            esac
            options="--listen --input --output --headphones --key-file --peers --state-dir --help"
            COMPREPLY=( $(compgen -W "$options" -- "$cur") )
            ;;
        pair|join)
            case "$prev" in
                --code|--listen|--input|--output|--peers)
                    return 0
                    ;;
            esac
            options="--code --listen --input --output --headphones --state-dir --help"
            COMPREPLY=( $(compgen -W "$options" -- "$cur") )
            ;;
        status|watch)
            COMPREPLY=( $(compgen -W "--json --state-dir --help" -- "$cur") )
            ;;
        update)
            case "$prev" in
                --archive|--checksums)
                    COMPREPLY=( $(compgen -f -- "$cur") )
                    return 0
                    ;;
            esac
            options="--version --archive --checksums --state-dir --help"
            COMPREPLY=( $(compgen -W "$options" -- "$cur") )
            ;;
        mute|unmute)
            COMPREPLY=( $(compgen -W "input output --state-dir --help" -- "$cur") )
            ;;
        invite|version)
            COMPREPLY=( $(compgen -W "--state-dir --help" -- "$cur") )
            ;;
    esac
}

complete -o bashdefault -o default -F _wirectl_talk_completion wirectl wirectl-talk
