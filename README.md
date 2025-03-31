# MediaSink.Go

![License](https://img.shields.io/badge/license-AGPL--v3-blue)
![Go Version](https://img.shields.io/badge/Go-1.x-blue)
[![Build Status](https://teamcity.sedrad.com/app/rest/builds/buildType:(id:MediaSinkGo_Build)/statusIcon)](https://teamcity.sedrad.com/viewType.html?buildTypeId=MediaSinkGo_Build&guest=1)
![Build](https://img.shields.io/github/actions/workflow/status/srad/MediaSink.Go/build.yml)

MediaSink.Go is a powerful web-based media management and streaming server written in Go. It provides automated stream recording capabilities and a REST API for video editing, making it an ideal solution for media-heavy applications. The project also includes a web-based user interface, available at [MediaSink.Vue](https://github.com/srad/MediaSink.Vue), which offers an intuitive way to manage media files and interact with the server.

## Features
- **Media Management**: Scans all media and generate previews and organizes them. Allows bookmarking folders, channel, media items, and tagging the media.
- **Automated Stream Recording**: Capture and store video streams automatically.
- **REST API for Video Editing**: Perform video editing tasks programmatically.
- **Web-Based User Interface**: Manage media files and interact with the server using [MediaSink.Vue](https://github.com/srad/MediaSink.Vue).
- **Scalable & Lightweight**: Optimized for performance with a minimal resource footprint.
- **Easy Integration**: RESTful API for seamless integration with other applications.
- **Disaster Recovery**: The system has which means that if the system crashes during recordings or job execution, it will try to recover all recordings on the next launch.

## Installation

This is mainly for development purposes. In production you'd use the Docker image.

### Prerequisites
- Go 1.x or later
- FFmpeg (for video processing capabilities)
- youtube-dl
- FFprobe
- SQLite 3

If you run the application outside of Docker, you must manually install FFmpeg, youtube-dl, FFprobe, and SQLite 3.
- Go 1.x or later
- FFmpeg (for video processing capabilities)

Debian setup:

```sh
sudo apt update && sudo apt install -y wget ffmpeg youtube-dl sqlite3
```

Go setup (replace with latest version):

```sh
sudo apt update
sudo apt install -y wget
wget https://golang.org/dl/go1.20.5.linux-amd64.tar.gz  # Replace with the latest version if needed
sudo tar -C /usr/local -xvzf go1.20.5.linux-amd64.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc
source ~/.bashrc
```

### Clone the Repository
```sh
git clone https://github.com/srad/MediaSink.Go.git
cd MediaSink.Go
```

Run a build: 

```sh
./build.sh
```

The configuration file is located at `conf/app.yml` and can be modified to customize the server settings.

### Build the Project

```sh
./run.sh
```

### Run Test

```sh
go test ./...
```

## Usage

### API Endpoints
MediaSink.Go provides a REST API to manage video recording and editing. Below are some key endpoints:
For a complete API reference, check the [API Documentation](https://github.com/srad/MediaSink.Go/wiki/API-Docs).

## Docker

The official Docker image for MediaSink.Go is available on [Docker Hub](https://hub.docker.com/r/sedrad/mediasink-server). It contains all necessary dependencies, including FFmpeg, Sqlite 3, and does not depend on any other service, so you can run it without any additional setup.

#### Docker Compose Setup

```yaml
services:
  # Static files are served by nginx
  recordings:
    image: "nginx"
    environment:
      - TZ=Europe/Berlin
    volumes:
      - ${DATA_PATH}:/usr/share/nginx/html:ro
      - "./nginx.conf:/etc/nginx/nginx.conf:ro"
    ports:
      - "4000:80"
  
  mediasink-server:
    image: sedrad/mediasink-server
    environment:
      - TZ=${TIMEZONE}
    volumes:
      - ${DATA_PATH}:/recordings
      - ${DISK}:/disk
    ports:
      - "3000:3000"
```

`.env` file:

```
# Timezone for the server
TIMEZONE=Europe/Berlin

# Path where recorded videos will be stored
DATA_PATH=/path/to/recordings

# Path to the disk root. This is used to query the disk status.
DISK=/mnt/disk1
```

#### Deploy

```sh
docker-compose --env-file .env up -d
```

## Contributing
We welcome contributions! To get started:
1. Fork the repository.
2. Create a new branch.
3. Make your changes and commit them.
4. Submit a pull request.

## License
MediaSink.Go is dual-licensed under the GNU Affero General Public License (AGPL) and a commercial license.

- **Open-Source Use (AGPL License)**: MediaSink.Go is free to use, modify, and distribute under the terms of the [GNU AGPL v3](https://www.gnu.org/licenses/agpl-3.0.html). Any modifications and derivative works must also be open-sourced under the same license.
- **Commercial Use**: Companies that wish to use MediaSink.Go without AGPL restrictions must obtain a commercial license. For more details, please refer to the [LICENSE](LICENSE) file or contact us for licensing inquiries.
MediaSink.Go is available for free for non-profit and educational institutions. However, a commercial license is required for companies. For more details, please refer to the [LICENSE](LICENSE) file or contact us for licensing inquiries.
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contact
For issues and feature requests, please use the [GitHub Issues](https://github.com/srad/MediaSink.Go/issues) section.

## Notes & Limitations

1. All streaming services allow only a limited number of request made by each client.
If this limit is exceeded the client will be temporarily or permanently blocked.
In order to circumvent this issue, the application does strictly control the 
timing between each request. However, this might cause that the recording will only start
recording after a few minutes and not instantly.
2. The system has disaster recovery which means that if the system crashes during recordings,
it will try to recover all recordings on the next launch. However, due to the nature of
streaming videos and the crashing behavior, the video files might get corrupted.
In this case they will be automatically delete from the system, after they have been
checked for integrity. Otherwise, they are added to the library.


---
Star the repo if you find it useful! ⭐
