# -----------------------------------------------------------------------------------
# Stage: Common Build Environment
# -----------------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1-bookworm AS builder_base

ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV TZ=Europe/Berlin
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Setup custom apt sources if packages from contrib/non-free are needed during builds
RUN { \
      echo "deb http://ftp.de.debian.org/debian/ bookworm main contrib non-free"; \
      echo "deb-src http://ftp.de.debian.org/debian/ bookworm main contrib non-free"; \
    } | tee /etc/apt/sources.list.d/custom-sources.list && \
    apt-get update && apt-get upgrade -y && \
    # Install locales and common tools needed by multiple build stages
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
      locales \
      ca-certificates \
      git \
      wget \
      build-essential \
      pkg-config \
      python3 \
      python3-pip \
      python-is-python3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8
ENV LANG=en_US.UTF-8

# -----------------------------------------------------------------------------------
# Stage: yt-dlp Installer (Replaces Youtube-DL Builder)
# -----------------------------------------------------------------------------------
FROM builder_base AS yt_dlp_builder
# builder_base already has python, wget, curl

# Option 1: Download pre-built binary
ARG TARGETARCH
RUN YTDLP_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp" && \
    if [ "$TARGETARCH" = "arm64" ]; then \
      YTDLP_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64"; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
      # The default URL is usually x86_64, but being explicit is fine
      YTDLP_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"; \
    fi && \
    echo "Downloading yt-dlp for $TARGETARCH from $YTDLP_URL" && \
    curl -SL "$YTDLP_URL" -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Add a symlink if you want to call it as youtube-dl for compatibility
RUN ln -s /usr/local/bin/yt-dlp /usr/local/bin/youtube-dl

# Verify installation (optional but good)
RUN yt-dlp --version

# # Option 2: Install using pip (if you prefer this method)
# # Ensure python3-pip is installed in builder_base (it is)
# RUN pip3 install --upgrade yt-dlp
# # This will typically install yt-dlp to a location like /usr/local/bin/yt-dlp
# # You might need to ensure this location is in PATH or copy it if needed.
# # Check with: RUN which yt-dlp
# # Add symlink if needed:
# # RUN ln -s $(which yt-dlp) /usr/local/bin/youtube-dl

# -----------------------------------------------------------------------------------
# Stage: FFMPEG Builder
# -----------------------------------------------------------------------------------
FROM builder_base AS ffmpeg_builder
# builder_base has build-essential, git, wget

# Install FFMPEG specific build dependencies
# Custom sources are already configured in builder_base
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
      nasm \
      g++-x86-64-linux-gnu \
      gcc-x86-64-linux-gnu \
      libc6-dev-amd64-cross \
      binutils \
      libunistring-dev \
      libx264-dev \
      libx265-dev \
      libnuma-dev \
      libvpx-dev \
      libfaac-dev \
      libfdk-aac-dev \
      libmp3lame-dev \
      libopus-dev \
      autoconf \
      automake \
      cmake \
      libass-dev \
      libfreetype6-dev \
      libgnutls28-dev \
      libsdl2-dev \
      libtool \
      libva-dev \
      libvdpau-dev \
      libvorbis-dev \
      libxcb1-dev \
      libxcb-shm0-dev \
      libxcb-xfixes0-dev \
      meson \
      ninja-build \
      texinfo \
      yasm \
      zlib1g-dev \
      make \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN git clone --branch release/7.1 --depth 1 https://git.ffmpeg.org/ffmpeg.git /ffmpeg
WORKDIR /ffmpeg

# Configure for static linking where possible to reduce runtime dependencies
RUN ./configure \
  --prefix=/usr/local \
  --pkg-config-flags="--static" \
  --extra-libs="-lpthread -lm" \
  --ld="g++" \
  --enable-gpl \
  --enable-gnutls \
  --enable-libass \
  --enable-libfdk-aac \
  --enable-libfreetype \
  --enable-libmp3lame \
  --enable-libopus \
  --enable-libvorbis \
  --enable-libvpx \
  --enable-libx264 \
  --enable-libx265 \
  --enable-nonfree \
  --disable-doc \
  --enable-static \
  --disable-shared

RUN make -j$(nproc)
RUN make install
RUN make distclean
# ffmpeg and ffprobe should now be in /usr/local/bin/

# -----------------------------------------------------------------------------------
# Stage: Go Application Builder
# -----------------------------------------------------------------------------------
FROM builder_base AS app_builder
# builder_base has Go, git, build-essential, python for potential build scripts

ARG VERSION
ARG COMMIT
# TARGETOS and TARGETARCH are inherited from builder_base or can be re-declared

# Install Go app specific CGO dependencies
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
      libsqlite3-dev \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /app
RUN mkdir -p docs

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN chmod a+x wait-for-it.sh
RUN chmod a+x docker-entrypoint.sh

COPY conf/app.docker.yml conf/app.yml
# Assets like fonts will be copied in the final stage

RUN go install github.com/swaggo/swag/cmd/swag@latest
# Ensure swag is in PATH or use full path. $(go env GOPATH)/bin is usually in PATH.
RUN $(go env GOPATH)/bin/swag init

RUN go mod tidy
RUN go mod vendor

ENV CGO_ENABLED=1
# GOOS and GOARCH are set using ARGs inherited or re-declared from global scope passed during build
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

# ARM64 specific compilation (ensure relevant cross-compilers are in builder_base if needed)
# RUN if [ "$TARGETPLATFORM" = "linux/arm64" ]; then apt install gccgo-arm-linux-gnueabihf binutils-arm-linux-gnueabi gcc-aarch64-linux-gnu -y; fi
# RUN if [ "$TARGETPLATFORM" = "linux/arm64" ]; then GOARCH='arm' GOHOSTARCH='arm' CC=arm-linux-gnueabihf-gcc GOGCCFLAGS="-march=armv8-a" CXX=arm-linux-gnueabi-g++ go build -gcflags="-l -N" -o ./main; else go build -o /app/main -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT'"; fi

RUN go build -o /app/main -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT'"


# -----------------------------------------------------------------------------------
# Stage: Final Runtime Image
# -----------------------------------------------------------------------------------
FROM debian:bookworm-slim AS final

# Inherit ARGs needed for runtime decisions or labels if any (not strictly needed for this config)
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV TZ=Europe/Berlin
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN echo "deb http://ftp.de.debian.org/debian/ bookworm main contrib non-free" | tee -a /etc/apt/sources.list
RUN echo "deb-src http://ftp.de.debian.org/debian/ bookworm main contrib non-free" | tee -a /etc/apt/sources.list

RUN apt-get update && apt-get upgrade -y
RUN DEBIAN_FRONTEND=noninteractive apt-get install build-essential libsqlite3-dev sqlite3 python3 python3-pip locales -y

RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8
ENV LANG=en_US.UTF-8

RUN apt-get -y install \
      libx264-dev \
      libx265-dev \
      libnuma-dev \
      libvpx-dev \
      libfaac-dev \
      libfdk-aac-dev \
      libmp3lame-dev \
      libopus-dev \
      libass-dev \
      libfreetype6-dev \
      libgnutls28-dev \
      libsdl2-dev \
      libtool \
      libva-dev \
      libvdpau-dev \
      libvorbis-dev \
      libxcb1-dev \
      libxcb-shm0-dev \
      libxcb-xfixes0-dev \
      meson \
      ninja-build \
      pkg-config \
      texinfo \
      wget \
      yasm \
      zlib1g-dev

WORKDIR /app

# Copy built artifacts from builder stages
COPY --from=app_builder /app/main /app/main
COPY --from=app_builder /app/docs ./docs

COPY --from=yt_dlp_builder /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp

COPY --from=ffmpeg_builder /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg_builder /usr/local/bin/ffprobe /usr/local/bin/ffprobe

# Copy assets
COPY ./assets/DMMono-Regular.ttf /usr/share/fonts/truetype/
COPY ./assets/live.jpg ./assets/
COPY ./docker-entrypoint.sh ./docker-entrypoint.sh
COPY ./wait-for-it.sh ./wait-for-it.sh
COPY ./conf/app.docker.yml conf/app.yml
RUN fc-cache -fv

# Ensure scripts are executable
RUN chmod +x /app/docker-entrypoint.sh
RUN chmod +x /app/wait-for-it.sh

RUN mkdir -p /recordings
RUN mkdir -p /disk

ENV SECRET ""

EXPOSE 3000

ENTRYPOINT ["/app/docker-entrypoint.sh"]

CMD ["./main"]