

# Usage

```
conda activate eia-mbs && pymodbus.console tcp --host 127.0.0.1 --port 1502
```

```
client.write_register address=40001 value=10
```

```
> client.read_holding_registers address=40001 count=2
{
    "registers": [
        4,
        0
    ]
}
```