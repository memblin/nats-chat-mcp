# nats-chat-mcp — project tasks. Run `just` with no recipe to see this list.

# Name of the console binary, and the system-PATH symlink we point at it.
console-bin := "nats-chat-console"
symlink := "/usr/local/bin/" + console-bin

# Show available recipes.
default:
    @just --list

# Build the MCP server (TypeScript -> dist/).
build-nats-chat:
    npm run build

# Build the console TUI (Go) into console/.
build-nats-chat-console:
    cd console && go build -o {{console-bin}} ./cmd/nats-chat-console

# Install the console for your user, then symlink it into /usr/local/bin via sudo if missing.
install:
    #!/usr/bin/env bash
    set -euo pipefail
    cd console
    echo "Installing {{console-bin}} into $(go env GOPATH)/bin ..."
    go install ./cmd/nats-chat-console
    target="$(go env GOPATH)/bin/{{console-bin}}"
    if [[ -e "{{symlink}}" || -L "{{symlink}}" ]]; then
        echo "Symlink {{symlink}} already present — leaving it as is."
    else
        echo "Creating symlink {{symlink}} -> ${target} (needs sudo) ..."
        if sudo ln -s "${target}" "{{symlink}}"; then
            echo "Linked {{symlink}}."
        else
            echo "Could not create {{symlink}} via sudo." >&2
            echo "Add $(go env GOPATH)/bin to your PATH, or create the link manually." >&2
        fi
    fi
