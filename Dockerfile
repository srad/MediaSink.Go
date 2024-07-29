FROM --platform=$BUILDPLATFORM golang:1-bookworm

ENV TZ=Europe/Berlin
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN echo "deb http://ftp.de.debian.org/debian/ bookworm main contrib non-free" | tee -a /etc/apt/sources.list
RUN echo "deb-src http://ftp.de.debian.org/debian/ bookworm main contrib non-free" | tee -a /etc/apt/sources.list

RUN apt-get update && apt-get upgrade -y
RUN DEBIAN_FRONTEND=noninteractive apt-get install build-essential libsqlite3-dev sqlite3 python3 python3-pip locales -y
RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8
ENV LANG en_US.UTF-8

RUN pip install youtube-dl --break-system-packages

#RUN wget -q https://yt-dl.org/downloads/latest/youtube-dl -O /usr/local/bin/youtube-dl
#RUN chmod a+rx /usr/local/bin/youtube-dl

# Cross compilation issues since some unknown version of debian or go, unclear.
# https://github.com/confluentinc/confluent-kafka-go/issues/898
RUN apt install g++-x86-64-linux-gnu libc6-dev-amd64-cross -y

# Start ffmpeg build
RUN apt install -y nasm git gcc binutils libunistring-dev libx264-dev libx265-dev libnuma-dev libvpx-dev libfaac-dev libfdk-aac-dev libmp3lame-dev libopus-dev
RUN apt -y install \
      autoconf \
      automake \
      build-essential \
      cmake \
      git-core \
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

RUN git clone https://git.ffmpeg.org/ffmpeg.git /ffmpeg
WORKDIR /ffmpeg

RUN ./configure \
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
  --disable-doc

RUN make
RUN make install
RUN make distclean
RUN apt autoremove -y
# End ffmpeg build

RUN mkdir -p /recordings
RUN mkdir -p /disk

WORKDIR /app
RUN mkdir -p docs

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN chmod a+x wait-for-it.sh

COPY conf/app.docker.yml conf/app.yml
COPY ./assets/DMMono-Regular.ttf /usr/share/fonts/truetype/

RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init

RUN go mod tidy
RUN go mod vendor

# https://github.com/mattn/go-sqlite3/issues/803
RUN GOFLAGS="-g -O2 -Wno-return-local-addr"

ENV CGO_ENABLED=1
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

ARG TARGETOS=TARGETARCH
RUN go build -o ./streamsink

EXPOSE 3000

CMD [ "./streamsink" ]
