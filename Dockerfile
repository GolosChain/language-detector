From ubuntu:14.04

ADD ./ /language-detector

WORKDIR /language-detector

RUN apt-get update && apt-get install -y software-properties-common && \
    add-apt-repository ppa:ubuntu-lxc/lxd-stable && apt-get update && apt-get -y dist-upgrade && \
    apt-get install -y build-essential curl git golang && \
    cd /language-detector/cld2/internal/ && ./compile_libs.sh && cp *.so ../../ && \
    cd /language-detector && g++ -Wall -c wrapper.cc -o wrapper.o && ar rvs wrapper.a wrapper.o && \
    LD_LIBRARY_PATH=. make && \
    apt-get remove -y golang git curl build-essential software-properties-common && apt-get autoremove -y

EXPOSE 3000
EXPOSE 30000

CMD /language-detector/language-detector >> /language-detector/log/language-detector.log 
