FROM golang
MAINTAINER Stewart Grant sgrant09@cs.ubc.ca

WORKDIR /
RUN apt-get update
EXPOSE 3000


#Install Dviz
RUN go get github.com/wantonsolutions/Dviz

ENTRYPOINT bash
    #&& cd /go/src/github.com/wantonsolutions/Dviz \
    #&& go install

#Install Pangaea Server
#RUN go get github.com/zipengliu/PangaeaServer \
#    && cd /go/src/github.com/zipengliu/PangaeaServer \
#    && node app.js

#Install Pangaea
#RUN go get github.com/zipengliu/Pangaea \
#    && cd /go/src/github.com/zipengliu/Pangaea \
#    && npm start
