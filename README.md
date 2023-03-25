# saleae
Tools to work with Saleae's Logic 2 software.

### SPI Analyzer
The SPI analyzer under [`analyzers`](./analyzers/spi.go) extends the functionality
of the provided SPI protocol analyzer in the Logic 2 software. It will group
transactions using the `enable` signal. This is key to working with
devices that communicate on an enable-line toggle basis such as the CYW43439 and 
other, if not most, SPI devices.

An example on how to use it can be found under [`examples_test.go`](./examples_test.go)