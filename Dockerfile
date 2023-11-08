ARG DISTRO=debian
ARG DISTRO_VERSION=bookworm-slim

FROM ${DISTRO}:${DISTRO_VERSION}

# ARG DEBIAN_FRONTEND=noninteractive

RUN <<EOF
set -eux
apt-get update
# apt-get upgrade -y
apt-get install -y build-essential software-properties-common cmake git # wget
# apt-get install -y libssl-dev libffi-dev libbz2-dev libncurses-dev libncursesw5-dev libgdbm-dev liblzma-dev libsqlite3-dev tk-dev libgdbm-compat-dev libreadline-dev
EOF

# WORKDIR /opt/python
# RUN <<EOF
# set -eux
# wget -qO- https://www.python.org/ftp/python/3.11.6/Python-3.11.6.tgz | tar --extract --verbose --gzip --strip-components=1
# ./configure
# make -j$(nproc)
# make install
# python3 --version
# EOF

# RUN rm -rf /opt/python

#COPY --from=python:3.11-slim-bullseye /usr/local/bin /usr/local/bin

COPY --from=node:20-${DISTRO_VERSION} /usr/local/bin/node /usr/local/bin/node
COPY --from=node:20-${DISTRO_VERSION} /usr/local/include/node /usr/local/include/node
COPY --from=node:20-${DISTRO_VERSION} /usr/local/lib/node_modules /usr/local/lib/node_modules

COPY --from=python:-${DISTRO_VERSION} /usr/local/bin/python3 /usr/local/bin/python3
COPY --from=python-${DISTRO_VERSION} /usr/local/bin/python3.11 /usr/local/bin/python3.11
COPY --from=python-${DISTRO_VERSION} /usr/local/bin/python /usr/local/bin/python

COPY --from=python-${DISTRO_VERSION} /usr/local/lib/python3.8 /usr/local/lib/python3.8
COPY --from=python-${DISTRO_VERSION} /usr/local/lib/libpython3.11.so.1.0 /usr/local/lib/libpython3.11.so.1.0
COPY --from=python-${DISTRO_VERSION} /usr/local/lib/libpython3.so /usr/local/lib/libpython3.so


# WORKDIR /opt/node
# RUN <<EOF
# set -eux
# wget -qO- https://nodejs.org/dist/latest-v20.x/node-v20.9.0.tar.gz | tar --extract --verbose --gzip --strip-components=1
# ./configure
# make -j$(nproc)
# make install
# node --version
# EOF

# RUN rm -rf /opt/node

COPY --from=node:20-slim /usr/local/bin /usr/local/bin
COPY --from=node:20-slim /usr/local/lib/node_modules /usr/local/lib/node_modules

RUN <<-EOF
set -eux
python3 -m pip install conan
conan profile detect

cat <<PROFILE > ~/.conan2/profiles/webassembly
include(default)

[settings]
arch=wasm
os=Emscripten

[tool_requires]
*: emsdk/3.1.44
PROFILE
EOF

WORKDIR /opt
RUN git clone --depth 1 https://github.com/carimbolabs/carimbo.git
WORKDIR /opt/carimbo/build
RUN <<EOF
set -eux
conan install ..  --output-folder=. --build=missing --profile=webassembly --settings compiler.cppstd=20 --settings build_type=Release
cmake .. -DCMAKE_TOOLCHAIN_FILE=conan_toolchain.cmake -DCMAKE_BUILD_TYPE=Release
# cmake --build . --config Release
EOF

# FROM golang:1.21
# WORKDIR /opt
# COPY go.mod .
# COPY go.sum .
# RUN go mod download
# COPY . .
# RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o app

# FROM gcr.io/distroless/static-debian12
# COPY --from=0 /opt/app /
# CMD ["/app"]