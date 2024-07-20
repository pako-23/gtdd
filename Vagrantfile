VM_CPUS = 6
VM_MEMORY = 8192

Vagrant.configure('2') do |config|
  $script = <<-SHELL
        cd /etc/apt/sources.list.d/
        wget http://www.mirbsd.org/~tg/Debs/sources.txt/wtf-bookworm.sources
        cd /opt
        wget -O- https://archive.apache.org/dist/maven/maven-3/3.6.1/binaries/apache-maven-3.6.1-bin.tar.gz |
sudo tar -xzv
        echo 'export PATH="$PATH:/opt/apache-maven-3.6.1/bin/"' >> /etc/profile
        echo 'export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64' >> /etc/profile
        apt update
        apt upgrade -y
        apt-get install -y ca-certificates curl gnupg make openjdk-8-jdk
	cd /tmp
	wget https://go.dev/dl/go1.22.1.linux-amd64.tar.gz
	tar -C /usr/local -xzf go1.22.1.linux-amd64.tar.gz
	echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/golang.sh
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        chmod a+r /etc/apt/keyrings/docker.gpg
        echo \
            "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
            "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
        tee /etc/apt/sources.list.d/docker.list > /dev/null
        apt update
        apt-get install -y  docker-ce docker-ce-cli containerd.io cloud-utils
        usermod -aG docker vagrant
        curl -SL https://github.com/docker/compose/releases/download/v2.23.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
        chmod +x /usr/local/bin/docker-compose
        growpart /dev/vda 1
        resize2fs /dev/vda1
    SHELL

    config.vm.provider 'libvirt' do |v|
        v.machine_virtual_size = 50
        v.memory = VM_MEMORY
        v.cpus = VM_CPUS
    end

    config.vm.box = 'debian/bookworm64'
    config.vm.provision :shell, inline: $script, privileged: true
end
