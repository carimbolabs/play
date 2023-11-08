FROM ubuntu:jammy

# RUN apt-get update && apt-get install -y \
#   build-essential \
#   cmake \
#   git \
#   python3 \
#   python3-pip \
#   wget \
#   zip

RUN <<EOF
apt-get update
apt-get upgrade -y
apt-get install -y build-essential cmake git python3 python3-pip wget zip
pip install conan
conan profile detect
EOF

#RUN pip install conan
#RUN conan profile detect
# RUN conan profile update settings.compiler.libcxx=libstdc++17 default
#RUN cat ~/.conan2/profiles/default
#RUN conan create --profile webassembly --build=missing  ~/.conan/profiles/webassembly
#RUN conan profile update settings.arch=wasm settings.os=Emscripten tools-requires=emsdk/3.1.44 default
#RUN cat ~/.conan/profiles/webassembly

#~/.conan2/profiles/wasm

WORKDIR /opt

#RUN sleep infinity

# printf "[settings]\narch=x86\nbuild_type=Debug\n...." > /home/user/.conan/profiles/default