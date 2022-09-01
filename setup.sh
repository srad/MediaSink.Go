#!/bin/bash

echo ########################################################
echo Installing youtube-dl and compiling ffmpeg
echo ########################################################
echo

apt update
apt upgrade
apt install -y sqlite3 nasm python git gcc binutils

# youtube dl
wget -q https://yt-dl.org/downloads/latest/youtube-dl -O /usr/local/bin/youtube-dl
chmod a+rx /usr/local/bin/youtube-dl
hash -r

# ffmpeg
apt-get update -qq && apt-get -y install \
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

apt-get install libunistring-dev libx264-dev libx265-dev libnuma-dev libvpx-dev libfdk-aac-dev libmp3lame-dev libopus-dev -y

# Font used by ffmpeg
cp ./assets/DMMono-Regular.ttf /usr/share/fonts/truetype/

mkdir -p bin
cd bin
git clone https://git.ffmpeg.org/ffmpeg.git ffmpeg
cd ffmpeg

./configure \
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
  --enable-nonfree

make
make install
make distclean