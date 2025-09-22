#!/bin/sh

set -e

DIR=$(pwd)
LIBGIT2_PATH=$DIR/libgit2

# First ensure that libgit2 source tree is available
if [ ! -d libgit2 ]
then
    ./scripts/import_libgit2.sh
fi

mkdir -p $LIBGIT2_PATH



cd $HOME
wget https://www.openssl.org/source/openssl-3.3.2.tar.gz
tar xzf openssl-3.3.2.tar.gz
cd openssl-3.3.2
./Configure linux-x86_64 no-shared --prefix=$HOME/static/openssl
make -j$(nproc)
make install_sw
cd $HOME
wget https://www.libssh2.org/download/libssh2-1.11.0.tar.gz
tar xzf libssh2-1.11.0.tar.gz
cd libssh2-1.11.0
mkdir build && cd build
cmake .. -DBUILD_SHARED_LIBS=OFF -DCMAKE_INSTALL_PREFIX=$HOME/static/libssh2 \
         -DOPENSSL_ROOT_DIR=$HOME/static/openssl
cmake --build . --target install

cd $LIBGIT2_PATH
mkdir -p install/lib
mkdir -p build
cd build
cmake -DTHREADSAFE=ON \
      -DBUILD_CLAR=OFF \
      -DBUILD_SHARED_LIBS=OFF \
      -DCMAKE_C_FLAGS=-fPIC \
      -DCMAKE_BUILD_TYPE="RelWithDebInfo" \
      -DCMAKE_PREFIX_PATH="$HOME/static/openssl;$HOME/static/libssh2" \
      -DCMAKE_INSTALL_PREFIX=../install \
      -DUSE_SSH=ON \
      -DUSE_HTTPS=OpenSSL \
      -DCURL=OFF \
      ..

cmake --build .
make -j2 install

cd $DIR
