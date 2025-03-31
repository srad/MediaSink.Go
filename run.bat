@echo off

set "filePath=.\conf\app.yml"

if not exist "%filePath%" (
    echo The file does not exist. Creating config file ...
    copy .\conf\app.default.yml  .\conf\app.yml
)

md recordings

set VERSION="0.0.1-dev"
set COMMIT="dev"

.\build.bat

set DB_FILENAME="mediasink.sqlite3"
set DATA_DIR=".previews"
set DATA_DISK="C:/"
set NET_ADAPTER="eth2"
set REC_PATH="recordings"

.\main.exe