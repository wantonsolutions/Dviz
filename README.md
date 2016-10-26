# Dviz
Distributed State Visualizer

Dviz visualises the states of distibuted systems by analyzing a log of their states. The states of systems are logged and analyzed using [Dinv](https://bitbucket.org/bestchai/dinv), and visualizes them as a line graph using [gnuplot](http://www.gnuplot.info/).

## Heat Plot Rendering of 5 node raft
![](https://github.com/wantonsolutions/Dviz/blob/master/3draft.png)

## Line Graph of Proxy Server
![](https://github.com/wantonsolutions/Dviz/blob/master/2dlinn.png)

## Installing
Dviz can be installed using the go tool
```
go get github.com/wantonsolutions/Dviz
go install
```

## Dependencies
Dviz relies on 
[Dinv](https://bitbucket.org/bestchai/dinv)
[GoVector](https://github.com/arcaneiceman/GoVector)
[Gnuplot](http://www.gnuplot.info/)

## Running
Dviz generates its vizualization by analyzing a json file of a distributed systems execution. To generate the json run a Dinv instrumented system to generate logs. Then run
```
dinv -l -json *d.txt *g.txt
```

To generate a json of the systems distributed state. Dviz takes the json file as a command line argument.

```
Dviz output.json
```

The result of running Dviz is a graph of the distributed state. Try running Dviz on the json's in the [example](https://github.com/wantonsolutions/Dviz/tree/master/examples) directory for sample output.


