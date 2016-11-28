From ubuntu:14.04

ADD ./ /oh-augmentation-language-detector

WORKDIR /oh-augmentation-language-detector

RUN apt-get update && apt-get install -y software-properties-common && \
    add-apt-repository ppa:ubuntu-lxc/lxd-stable && apt-get update && apt-get -y dist-upgrade && \
    apt-get install -y build-essential curl git golang && \
    cd /oh-augmentation-language-detector/cld2/internal/ && ./compile_libs.sh && cp *.so ../../ && \
    cd /oh-augmentation-language-detector && g++ -Wall -c wrapper.cc -o wrapper.o && ar rvs wrapper.a wrapper.o && \
    LD_LIBRARY_PATH=. make && \
    apt-get remove -y golang git curl build-essential software-properties-common && apt-get autoremove -y

EXPOSE 3000
EXPOSE 30000

CMD /oh-augmentation-language-detector/oh-augmentation-language-detector >> /oh-augmentation-language-detector/log/oh-augmentation-language-detector.log 
