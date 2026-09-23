#compdef wirectl wirectl-talk

# zsh completion for wirectl talk and wirectl-talk.
# Load after enabling zsh completion with: autoload -Uz compinit && compinit
# Then load with: source <(wirectl talk completion zsh)

_wirectl_talk_completion() {
    local talk_index command_index command daemon_command plugin
    local -a candidates root_commands

    case "$words[1]" in
        wirectl)
            if (( CURRENT == 2 )); then
                root_commands=(talk version)
                for plugin in ${(k)commands}; do
                    [[ "$plugin" == wirectl-* ]] || continue
                    root_commands+=("${plugin#wirectl-}")
                done
                _describe 'command' root_commands
                return
            fi
            [[ "$words[2]" == talk ]] || return
            talk_index=2
            ;;
        wirectl-talk)
            talk_index=1
            ;;
        *)
            return
            ;;
    esac

    command_index=$((talk_index + 1))
    if (( CURRENT == command_index )); then
        candidates=(
            'completion:print bash or zsh completion script'
            'daemon:start, install, stop, or inspect background audio'
            'devices:list audio devices'
            'init:create room config'
            'input:send an audio file to peers'
            'levels:watch microphone and speaker audio levels'
            'record:record all peers or a selected peer'
            'test:test a local audio device with a live meter'
            'invite:generate a one-use pairing code'
            'join:join a room or start saved room audio'
            'mute:mute local input or output'
            'pair:save a room without starting audio'
            'status:show session status'
            'unmute:unmute local input or output'
            'update:install the latest release'
            'version:print version'
            'watch:watch session status'
        )
        _describe 'talk command' candidates
        return
    fi

    command="$words[$command_index]"
    case "$command" in
        completion)
            if (( CURRENT == command_index + 1 )); then
                candidates=(bash zsh)
                _describe 'shell' candidates
            else
                _message 'shell (bash or zsh)'
            fi
            ;;
        record|input|test)
            if (( CURRENT == command_index + 1 )); then
                case "$command" in
                    record) candidates=(start stop status) ;;
                    input) candidates=(start pause resume stop status) ;;
                    test) candidates=(input output) ;;
                esac
                _describe 'action' candidates
                return
            fi
            if [[ "$words[$((command_index+1))]" == start ]] && (( CURRENT == command_index + 2 )); then
                _files
                return
            fi
            _arguments -s \
                '--mode[file input mode]:mode:(replace mix)' \
                '--loop[repeat file input]' \
                '--peer[record one peer]:ID or address:' \
                '--device[test a specific device]:device ID:' \
                '--seconds[test duration]:seconds:' \
                '--json[output JSON]' \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        daemon)
            if (( CURRENT == command_index + 1 )); then
                candidates=(install start status stop)
                _describe 'daemon command' candidates
                return
            fi
            daemon_command="$words[$((command_index + 1))]"
            case "$daemon_command" in
                status)
                    _arguments -s \
                        '--json[output JSON for scripts]' \
                        '--state-dir[configuration directory]:directory:_files -/'
                    ;;
                install|start|stop)
                    _arguments -s \
                        '--state-dir[configuration directory]:directory:_files -/'
                    ;;
                *)
                    _arguments -s \
                        '--json[output JSON for scripts]' \
                        '--state-dir[configuration directory]:directory:_files -/'
                    ;;
            esac
            ;;
        devices)
            _arguments -s \
                '--json[output JSON for scripts]' \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        init)
            _arguments -s \
                '--listen[UDP listen address]:address:' \
                '--input[input device ID]:device ID:' \
                '--output[output device ID]:device ID:' \
                '--headphones[allow full duplex]' \
                '--key-file[read a room key from a private file]:file:_files' \
                '--peers[comma-separated reachable UDP addresses]:addresses:' \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        pair|join)
            _arguments -s \
                '--code[temporary six-digit invitation code]:code:' \
                '--listen[local UDP listen address]:address:' \
                '--input[input device ID]:device ID:' \
                '--output[output device ID]:device ID:' \
                '--headphones[allow full duplex]' \
                '--state-dir[configuration directory]:directory:_files -/' \
                '1:inviter host and port:'
            ;;
        status|watch)
            _arguments -s \
                '--json[output JSON for scripts]' \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        levels)
            _arguments -s \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        update)
            _arguments -s \
                '--version[release version]:version:' \
                '--archive[offline release archive]:archive:_files' \
                '--checksums[offline SHA256SUMS file]:file:_files' \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
        mute|unmute)
            _arguments -s \
                '--state-dir[configuration directory]:directory:_files -/' \
                '1:audio direction:(input output)'
            ;;
        invite|version)
            _arguments -s \
                '--state-dir[configuration directory]:directory:_files -/'
            ;;
    esac
}

# Autoloaded completion files must run on the first Tab as well as later calls.
# When sourced manually, only register the function.
if [[ $ZSH_EVAL_CONTEXT == *:file ]]; then
    if (( $+functions[compdef] )); then
        compdef _wirectl_talk_completion wirectl wirectl-talk
    fi
else
    _wirectl_talk_completion "$@"
fi
