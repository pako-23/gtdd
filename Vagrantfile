VM_CPUS = 10
VM_MEMORY = 12288

Vagrant.configure('2') do |config|
    $script = <<-SHELL
        sudo apt update
        sudo apt upgrade -y
        sudo apt-get install -y ca-certificates curl gnupg golang make
        sudo install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
        echo \
            "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
            "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
        sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        sudo apt update
        sudo apt-get install -y  docker-ce docker-ce-cli containerd.io cloud-utils
        sudo usermod -aG docker vagrant
        sudo curl -SL https://github.com/docker/compose/releases/download/v2.23.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
        sudo growpart /dev/vda 1
        sudo resize2fs /dev/vda1
    SHELL

    config.vm.provider 'libvirt' do |v|
        v.machine_virtual_size = 50
        v.memory = VM_MEMORY
        v.cpus = VM_CPUS
    end

    config.vm.box = 'debian/bookworm64'
    config.vm.provision :shell, inline: $script, privileged: false
end
