Get-Content .env | ForEach-Object {
    # Ignorar linhas vazias ou que começam com #
    if ($_ -match "^\s*([^#]\S*?)\s*=\s*(.*?)\s*$") {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim()

        # Remover aspas simples ou duplas do valor, se existirem
        $value = $value -replace '^"(.*)"$', '$1'  
        $value = $value -replace "^'(.*)'$", '$1'  

        [System.Environment]::SetEnvironmentVariable($name, $value)
    }
}
go run cmd/sender/main.go