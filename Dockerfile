FROM ubuntu:jammy

ARG DEBIAN_FRONTEND=noninteractive

RUN <<EOF
set -eux
apt-get update
apt-get upgrade -y
apt-get install -y build-essential software-properties-common cmake git wget
apt-get install -y libssl-dev libffi-dev libbz2-dev libncurses-dev libncursesw5-dev libgdbm-dev liblzma-dev libsqlite3-dev tk-dev libgdbm-compat-dev libreadline-dev
EOF

WORKDIR /opt/python
RUN <<EOF
set -eux
wget -qO- https://www.python.org/ftp/python/3.11.6/Python-3.11.6.tgz | tar --extract --verbose --gzip --strip-components=1
./configure
make -j$(nproc)
make install
python3 --version
EOF

RUN rm -rf /opt/python

WORKDIR /opt/node
RUN <<EOF
set -eux
wget -qO- https://nodejs.org/dist/latest-v20.x/node-v20.9.0.tar.gz | tar --extract --verbose --gzip --strip-components=1
./configure
make -j$(nproc)
make install
node --version
EOF

RUN rm -rf /opt/node

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
cmake --build . --config Release
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