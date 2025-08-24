
To verify your cloud config is valid YAML you can use validate-yaml.py.

To ensure the keys and values in your user data are correct, you can run:
```azure
sudo cloud-init schema --system --annotate
```