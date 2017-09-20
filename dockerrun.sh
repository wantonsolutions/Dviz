docker build -t dviz/1.0 -f ./docker/Dockerfile .
docker run -p 3000:5000 -ti dviz/1.0
