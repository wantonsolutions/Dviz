docker build -t dviz/1.0 -f ./docker/Dockerfile .
docker run -p 3000:3000 -p 23333:23333 -ti dviz/1.0
