FROM golang
MAINTAINER Stewart Grant sgrant09@cs.ubc.ca

WORKDIR /
RUN apt-get update
EXPOSE 3000


#Install Dviz
RUN go get github.com/wantonsolutions/Dviz \
    && cd /go/src/github.com/wantonsolutions/Dviz \
    && go install

#Install Pangaea Server
RUN apt-get install -y nodejs
RUN yes | apt-get install npm

RUN npm cache clean -f
RUN npm install -g n
RUN n stable
RUN ln -sf /usr/local/n/versions/node/8.4.0/bin/node /usr/bin/nodejs

RUN mkdir /go/src/github.com/zipengliu 
RUN git clone https://github.com/zipengliu/PangaeaServer /go/src/github.com/zipengliu/PangaeaServer

RUN npm install express
RUN npm install morgan
RUN npm install cors
RUN npm install express-ws-rpc
RUN npm install express-ws

RUN echo "tmp"

RUN git clone https://github.com/zipengliu/Pangaea /go/src/github.com/zipengliu/Pangaea

RUN cd /go/src/github.com/zipengliu/Pangaea \
    && npm install xmlhttprequest

RUN cd /go/src/github.com/zipengliu/Pangaea \
    && npm install

#RUN cd /go/src/github.com/zipengliu/Pangaea \
#    && npm start

ENTRYPOINT cd /go/src/github.com/zipengliu/PangaeaServer; node app.js >> /DvizServerLog.log & cd /go/src/github.com/zipengliu/Pangaea; npm start >> /DvizFrontEndLog.log & cd / ; bash
