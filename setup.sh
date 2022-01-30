#!/bin/bash

echo ########################################################
echo Installing youtube-dl and compiling ffmpeg
echo ########################################################
echo

sudo apt update
sudo apt install -y sqlite3 nasm python git

# youtube dl
sudo wget https://yt-dl.org/downloads/latest/youtube-dl -O /usr/local/bin/youtube-dl
sudo chmod a+rx /usr/local/bin/youtube-dl
hash -r

# ffmpeg
sudo apt-get update -qq && sudo apt-get -y install \
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

sudo apt-get install libunistring-dev libx264-dev libx265-dev libnuma-dev libvpx-dev libfdk-aac-dev libmp3lame-dev libopus-dev -y

# Font used by ffmpeg
sudo cp ./assets/DMMono-Regular.ttf /usr/share/fonts/truetype/

mkdir -f bin
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
sudo make install

# Define the port of the REST server
# Allow port to API and save the rule
sudo iptables -A INPUT -p tcp --dport 3000 -j ACCEPT
sudo apt-get install iptables-persistent -y
sudo netfilter-persistent save
