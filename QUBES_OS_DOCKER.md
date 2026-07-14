# Running Docker in Qubes OS

By default, Qubes OS AppVMs reset their root filesystem `/` to the template's state on every reboot. This means any Docker images you download or containers you create in `/var/lib/docker/` will be lost when the VM restarts.

Below are the two recommended methods to make Docker storage persistent in Qubes OS, followed by a daemonless alternative (Podman).

---

## Method 1: Using Qubes `bind-dirs` (Recommended)

This method keeps Docker’s default storage paths but instructs Qubes OS to persist the directories by binding them to the persistent `/rw` partition.

### Step 1: Install Docker in the TemplateVM
Open a terminal in your **TemplateVM** (e.g., `debian-12` or `fedora-39`) and install Docker:

```bash
# For Debian/Ubuntu templates:
sudo apt-get update
sudo apt-get install -y docker.io docker-compose

# For Fedora templates:
sudo dnf install -y docker docker-compose
```

Disable the Docker service inside the TemplateVM so it does not run during template updates:
```bash
sudo systemctl disable docker
sudo systemctl disable docker.socket
```

### Step 2: Configure Bind-Dirs in the TemplateVM
Create or edit `/etc/qubes-bind-dirs.d/50_user.conf` in the **TemplateVM**:
```bash
sudo mkdir -p /etc/qubes-bind-dirs.d
sudo nano /etc/qubes-bind-dirs.d/50_user.conf
```

Add the following directories to be persisted:
```bash
binds+=( '/var/lib/docker' )
binds+=( '/etc/docker' )
```
*(If you are using modern Docker with the `containerd` snapshotter, you may also want to add `/var/lib/containerd`).*

Shut down the TemplateVM:
```bash
sudo poweroff
```

### Step 3: Run and Start Docker in your AppVM
Start or restart your **AppVM**. The folders `/var/lib/docker` and `/etc/docker` will now persist automatically.

1. Add your user to the `docker` group to run commands without `sudo`:
   ```bash
   sudo usermod -aG docker user
   ```
2. Enable and start the Docker daemon inside the AppVM:
   ```bash
   sudo systemctl enable docker
   sudo systemctl start docker
   ```
   *(To automate starting the daemon on VM boot, you can add `sudo systemctl start docker` to `/rw/config/rc.local` inside your AppVM).*

---

## Method 2: Shifting Docker's `data-root`

This method moves Docker’s storage folder into your user's home directory (`/home/user/`), which Qubes natively persists across restarts.

### Step 1: Install Docker in the TemplateVM
Install Docker in the TemplateVM (as shown in Method 1) and disable the daemon, then shut down the TemplateVM.

### Step 2: Configure `daemon.json` in the AppVM
In your **AppVM**, create or modify the Docker daemon configuration file:
```bash
sudo mkdir -p /etc/docker
sudo nano /etc/docker/daemon.json
```

Paste the following JSON to change the storage root to `/home/user`:
```json
{
  "data-root": "/home/user/.docker-images",
  "group": "user",
  "storage-driver": "overlay2"
}
```

### Step 3: Start Docker
Add your user to the docker group and start the service:
```bash
sudo usermod -aG docker user
sudo systemctl start docker
```
*Docker will now store all images and container data under `/home/user/.docker-images`.*

---

## Alternative: Use Podman (Rootless & Zero-Config)

If you do not strictly require the Docker daemon, **Podman** is highly recommended for Qubes OS:
1. It is **daemonless** and runs fully rootless.
2. By default, it stores all images and container files in `/home/user/.local/share/containers/`.
3. Since it stores data in `/home/user/`, **it is naturally persistent** in Qubes OS AppVMs with zero configuration or bind-dirs required.

To install Podman in your TemplateVM:
```bash
# Debian:
sudo apt-get install -y podman

# Fedora:
sudo dnf install -y podman
```
Once installed, you can run container commands in your AppVM immediately using the `podman` command (e.g., `podman play kube` or standard run commands) without configuring daemon startup or storage directories.
