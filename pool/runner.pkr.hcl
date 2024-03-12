packer {
  required_plugins {
    tart = {
      version = ">= 1.2.0"
      source  = "github.com/cirruslabs/tart"
    }
  }
}

variable "xcode_xip" {
  type = string
}

variable "xcode_version" {
  type    = string
  default = "15.2"
}

variable "base_vm" {
  type    = string
  default = "sonoma-base-md"
}

variable "vm_name" {
  type    = string
  default = "sonoma-runner-md"
}

variable "android_sdk_tools_version" {
  type    = string
  default = "11076708" # https://developer.android.com/studio/#command-tools
}

variable "gha_version" {
  type    = string
  default = "2.313.0" # https://api.github.com/repos/actions/runner/releases/latest
}

source "tart-cli" "tart" {
  vm_base_name = "${var.base_vm}"
  vm_name      = "${var.vm_name}"
  cpu_count    = 4
  memory_gb    = 4
  disk_size_gb = 100
  ssh_password = "runner"
  ssh_username = "runner"
  ssh_timeout  = "120s"
}

build {
  sources = ["source.tart-cli.tart"]

  # configure brew
  provisioner "shell" {
    inline = [
      "/bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"",
      "echo \"export LANG=en_US.UTF-8\" >> ~/.zprofile",
      "echo 'eval \"$(/opt/homebrew/bin/brew shellenv)\"' >> ~/.zprofile",
      "echo \"export HOMEBREW_NO_AUTO_UPDATE=1\" >> ~/.zprofile",
      "echo \"export HOMEBREW_NO_INSTALL_CLEANUP=1\" >> ~/.zprofile",
      "source ~/.zprofile",
      "brew --version",
      "brew update",
      "brew upgrade",
    ]
  }

  # system tooling
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "brew install curl wget unzip zip ca-certificates jq htop",
    ]
  }

  # macOS software update
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "sudo softwareupdate --install-rosetta --agree-to-license"
    ]
  }

  # install the actions runner
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "cd $HOME",
      "rm -rf actions-runner",
      "mkdir actions-runner && cd actions-runner",
      "curl -O -L https://github.com/actions/runner/releases/download/v${var.gha_version}/actions-runner-osx-arm64-${var.gha_version}.tar.gz",
      "tar xzf ./actions-runner-osx-arm64-${var.gha_version}.tar.gz",
      "rm actions-runner-osx-arm64-${var.gha_version}.tar.gz",
    ]
  }

  # configure android tooling
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "brew install homebrew/cask-versions/temurin17",
      "echo 'export ANDROID_HOME=$HOME/android-sdk' >> ~/.zprofile",
      "echo 'export ANDROID_SDK_ROOT=$ANDROID_HOME' >> ~/.zprofile",
      "echo 'export PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator' >> ~/.zprofile",
      "source ~/.zprofile",
      "wget -q https://dl.google.com/android/repository/commandlinetools-mac-${var.android_sdk_tools_version}_latest.zip -O android-sdk-tools.zip",
      "mkdir -p $ANDROID_HOME/cmdline-tools/",
      "unzip -q android-sdk-tools.zip -d $ANDROID_HOME/cmdline-tools/",
      "rm android-sdk-tools.zip",
      "mv $ANDROID_HOME/cmdline-tools/cmdline-tools $ANDROID_HOME/cmdline-tools/latest",
      "yes | sdkmanager --licenses",
      "yes | sdkmanager 'platform-tools' 'platforms;android-33' 'build-tools;34.0.0' 'ndk;25.2.9519653'"
    ]
  }

  # get xcode
  provisioner "shell" {
    inline = [
      "sudo mkdir -p /usr/local/bin/",
      "echo 'export PATH=/usr/local/bin/:$PATH' >> ~/.zprofile",
      "source ~/.zprofile",
      "brew install xcodesorg/made/xcodes",
      "xcodes version",
      "wget '${var.xcode_xip}' -O /Users/runner/Downloads/Xcode_${var.xcode_version}.xip",
      "xcodes install ${var.xcode_version} --experimental-unxip --path /Users/runner/Downloads/Xcode_${var.xcode_version}.xip --select --empty-trash",
      "xcodebuild -downloadAllPlatforms",
      "xcodebuild -runFirstLaunch",
    ]
  }

  # dev tooling
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "brew install asdf",
      "echo \". $(brew --prefix asdf)/libexec/asdf.sh\" >> ~/.zprofile",
      "source ~/.zprofile",
      "asdf plugin add nodejs",
      "asdf plugin add java",
      "asdf plugin add ruby",
      "asdf plugin add python",
    ]
  }

  # # configure ruby
  # provisioner "shell" {
  #   inline = [
  #     "source ~/.zprofile",
  #     "asdf install ruby 3.2.3",
  #     "asdf global ruby 3.2.3",
  #     "gem update --system",
  #   ]
  # }

  # configure cocoapods system deps
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "brew install libimobiledevice ideviceinstaller ios-deploy carthage cocoapods",
      # "gem install cocoapods",
      # "gem uninstall --ignore-dependencies ffi && sudo gem install ffi -- --enable-libffi-alloc"
    ]
  }

  # configure flutter
  # provisioner "shell" {
  #   inline = [
  #     "source ~/.zprofile",
  #     "echo 'export FLUTTER_HOME=$HOME/flutter' >> ~/.zprofile",
  #     "echo 'export PATH=$HOME/flutter:$HOME/flutter/bin/:$HOME/flutter/bin/cache/dart-sdk/bin:$PATH' >> ~/.zprofile",
  #     "source ~/.zprofile",
  #     "git clone https://github.com/flutter/flutter.git $FLUTTER_HOME",
  #     "cd $FLUTTER_HOME",
  #     "git checkout stable",
  #     "flutter doctor --android-licenses",
  #     "flutter doctor",
  #     "flutter precache",
  #   ]
  # }

  # useful utils for mobile development
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "brew install graphicsmagick imagemagick",
      "brew install wix/brew/applesimutils"
    ]
  }

  # inspired by https://github.com/actions/runner-images/blob/fb3b6fd69957772c1596848e2daaec69eabca1bb/images/macos/provision/configuration/configure-machine.sh#L33-L61
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "sudo security delete-certificate -Z FF6797793A3CD798DC5B2ABEF56F73EDC9F83A64 /Library/Keychains/System.keychain",
      "curl -o add-certificate.swift https://raw.githubusercontent.com/actions/runner-images/fb3b6fd69957772c1596848e2daaec69eabca1bb/images/macos/provision/configuration/add-certificate.swift",
      "swiftc add-certificate.swift",
      "sudo mv ./add-certificate /usr/local/bin/add-certificate",
      "curl -o AppleWWDRCAG3.cer https://www.apple.com/certificateauthority/AppleWWDRCAG3.cer",
      "curl -o DeveloperIDG2CA.cer https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer",
      "sudo add-certificate AppleWWDRCAG3.cer",
      "sudo add-certificate DeveloperIDG2CA.cer",
      "rm add-certificate* *.cer"
    ]
  }

  # configure ssh
  provisioner "shell" {
    inline = [
      "mkdir -p ~/.ssh",
      "touch ~/.ssh/authorized_keys",
      "chmod 700 ~/.ssh",
      "chmod 600 ~/.ssh/authorized_keys",
    ]
  }
}
