#compdef englint

local -a commands
commands=(
  'help:show help'
  'scan:scan files for non-English text'
  'init:create default config file'
  'version:show version'
)

if (( CURRENT == 2 )); then
  _describe -t commands command commands
  return
fi

case "$words[2]" in
  scan)
    local -a scan_flags
    scan_flags=(
      '--config:path to config file'
      '--exclude:exclude glob pattern'
      '--include:include glob pattern'
      '--json:json output'
      '--fix:auto-fix placeholder'
      '--severity:default severity (error|warning)'
      '--no-color:disable color output'
      '--verbose:show all scanned files'
    )
    _describe -t flags flag scan_flags
    ;;
  init)
    local -a init_flags
    init_flags=(
      '--config:path to config file'
    )
    _describe -t flags flag init_flags
    ;;
  *)
    ;;
esac
