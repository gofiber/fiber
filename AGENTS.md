# AGENTS.md

## Startup script

- Fetch dependencies:  
  ```bash
  go get ./...
  ```
- Install CLI tools referenced in Makefile:  
  ```bash
  go install gotest.tools/gotestsum@latest                 # test runner
  go install golang.org/x/vuln/cmd/govulncheck@latest      # vulnerability scanner
  go install mvdan.cc/gofumpt@latest                       # code formatter
  go install github.com/tinylib/msgp@latest                # msgp codegen
  go install github.com/vburenin/ifacemaker@975a95966976eeb2d4365a7fb236e274c54da64c  # interface impls
  go install github.com/dkorunic/betteralign/cmd/betteralign@latest  # struct alignment
  go mod tidy                                              # clean up go.mod & go.sum
  ```
