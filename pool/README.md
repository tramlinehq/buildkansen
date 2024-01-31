# Building macOS images

The Xcode xip file is no longer available on the Apple Developer site without authentication, which means that the packer script can't directly download it. 

We've hosted it on our [Google Drive](https://drive.usercontent.google.com/download?id=1Xmf1WrxkAThDoQGvxE8Q3_4Jx13i4XHU&export=download&authuser=1&confirm=t&uuid=1d669d53-6e5c-4718-bf6e-ac8f235234d0&at=APZUnTUOFnuG5x973LxfhLqvK60w%3A1706657627430), a link to which can be supplied:

```bash
packer build -var "vm_name=sonoma-runner-md" -var "base_vm=sonoma-base-md" -var "<link to Xcode15.2.xip>" runner.pkr.hcl
```

To build the base image:


```bash
packer build -var "vm_name=sonoma-base-md" sonoma.pkr.hcl
```
