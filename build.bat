@echo off

go install github.com/swaggo/swag/cmd/swag@latest
swag init

go build -o ./main.exe -ldflags="-X 'main.Version=%VERSION%' -X 'main.Commit=%COMMIT%'" -mod=mod