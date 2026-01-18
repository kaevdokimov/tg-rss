# Makefile for Ansible TG-RSS deployment

.PHONY: help setup validate vault-init vault-edit vault-view vault-rekey check check-ansible deploy dry-run clean logs backup restore status ping facts inventory ci-setup ci-check ci-deploy ci-clean

# Default target
help:
	@echo "Ansible TG-RSS Infrastructure Management"
	@echo "========================================"
	@echo "Setup & Security:"
	@echo "  setup         - Initial setup and dependency check"
	@echo "  validate      - Comprehensive setup validation"
	@echo "  vault-init    - Initialize Ansible Vault"
	@echo "  vault-edit    - Edit encrypted vault variables"
	@echo "  vault-view    - View encrypted vault variables"
	@echo "  vault-rekey   - Change vault password"
	@echo ""
	@echo "Deployment:"
	@echo "  check         - Run pre-deployment checks"
	@echo "  check-ansible - Run Ansible-specific checks only"
	@echo "  deploy        - Full deployment with vault"
	@echo "  dry-run       - Dry run deployment (check mode)"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean         - Clean temporary files and caches"
	@echo "  logs          - Show recent deployment logs"
	@echo "  backup        - Create backup of current state"
	@echo "  restore       - Restore from backup"
	@echo "  status        - Show system status"
	@echo "  ping          - Test connectivity to hosts"
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup        # First time setup"
	@echo "  make vault-init   # Setup secrets"
	@echo "  make check        # Validate everything"
	@echo "  make deploy       # Deploy to production"
	@echo "  make dry-run     # Test deployment"

# Setup and dependencies
setup:
	@echo "🔧 Checking Ansible installation..."
	@command -v ansible >/dev/null 2>&1 || { echo "❌ Ansible not installed. Install with: pip install ansible"; exit 1; }
	@echo "✅ Ansible found: $$(ansible --version | head -1)"
	@echo ""
	@echo "🔧 Checking required collections..."
	@ansible-galaxy collection list | grep -q community.docker || ansible-galaxy collection install community.docker
	@ansible-galaxy collection list | grep -q community.general || ansible-galaxy collection install community.general
	@echo "✅ Collections ready"
	@echo ""
	@echo "🎉 Setup complete! Run 'make vault-init' to setup secrets."

vault-init:
	@echo "🔐 Initializing Ansible Vault..."
	@chmod +x scripts/vault_setup.sh
	@./scripts/vault_setup.sh

vault-edit:
	@echo "📝 Editing vault variables..."
	@ansible-vault edit inventory/group_vars/all/vault.yml --vault-password-file .vault_password

vault-view:
	@echo "👁️  Viewing vault variables..."
	@ansible-vault view inventory/group_vars/all/vault.yml --vault-password-file .vault_password

vault-rekey:
	@echo "🔑 Rekeying vault (changing password)..."
	@ansible-vault rekey inventory/group_vars/all/vault.yml --vault-password-file .vault_password

# Pre-deployment checks
check:
	@echo "🔍 Running comprehensive setup validation..."
	@./scripts/validate_setup.sh

validate:
	@echo "🔍 Validating setup..."
	@./scripts/validate_setup.sh

check-ansible:
	@echo "🔍 Running Ansible-specific checks..."
	@if [ -f inventory/hosts.ini ]; then \
		echo "  📋 Checking inventory..."; \
		echo "  ✅ Inventory exists"; \
	else \
		echo "  ⚠️ Inventory not found (normal for CI/CD)"; \
	fi
	@if [ -f .vault_password ]; then \
		echo "  🔐 Checking vault..."; \
		ansible-vault view inventory/group_vars/all/vault.yml --vault-password-file .vault_password >/dev/null 2>&1 || { echo "❌ Vault not accessible"; exit 1; }; \
		echo "  ✅ Vault accessible"; \
	else \
		echo "  ⚠️ Vault password not found (will use environment variable)"; \
	fi
	@echo "  📊 Checking syntax..."
	@ansible-playbook playbooks/server-setup.yml --syntax-check >/dev/null 2>&1 || { echo "❌ Syntax error in playbook"; exit 1; }
	@echo "  ✅ Syntax OK"
	@echo ""
	@echo "🎉 Ansible checks passed! Ready for deployment."

# Deployment
deploy:
	@echo "🚀 Starting deployment..."
	@if [ -f .vault_password ]; then \
		ansible-playbook -i inventory/hosts.ini playbooks/server-setup.yml --vault-password-file .vault_password; \
	else \
		echo "❌ Vault password file not found. Run 'make vault-init' first."; \
		exit 1; \
	fi

dry-run:
	@echo "🧪 Running dry-run (check mode)..."
	@ansible-playbook -i inventory/hosts.ini playbooks/server-setup.yml --check --diff --vault-password-file .vault_password

# Maintenance
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf logs/*.retry
	@rm -rf .ansible/tmp/
	@find . -name "*.retry" -delete
	@find . -name "__pycache__" -type d -exec rm -rf {} +
	@echo "✅ Cleanup complete"

logs:
	@echo "📋 Recent deployment logs:"
	@ls -la logs/ 2>/dev/null || echo "No logs directory found"
	@echo ""
	@echo "📄 Ansible log:"
	@tail -50 logs/ansible.log 2>/dev/null || echo "No ansible.log found"

backup:
	@echo "💾 Creating backup..."
	@mkdir -p backups
	@ansible -i inventory/hosts.ini tg_rss_servers -m shell -a "tar czf /tmp/backup-$$(date +%Y%m%d-%H%M%S).tar.gz /opt/tg-rss" --vault-password-file .vault_password
	@ansible -i inventory/hosts.ini tg_rss_servers -m fetch -a "src=/tmp/backup-*.tar.gz dest=backups/" --vault-password-file .vault_password
	@echo "✅ Backup created in backups/"

restore:
	@echo "🔄 Available backups:"
	@ls -la backups/ 2>/dev/null || echo "No backups found"
	@echo ""
	@echo "To restore, run:"
	@echo "ansible -i inventory/hosts.ini tg_rss_servers -m copy -a \"src=backups/BACKUP_FILE dest=/tmp/\" --vault-password-file .vault_password"
	@echo "ansible -i inventory/hosts.ini tg_rss_servers -m shell -a \"cd /opt && tar xzf /tmp/BACKUP_FILE\" --vault-password-file .vault_password"

# Utility commands
ping:
	@echo "🏓 Pinging hosts..."
	@ansible -i inventory/hosts.ini tg_rss_servers -m ping --vault-password-file .vault_password

facts:
	@echo "📊 Gathering facts..."
	@ansible -i inventory/hosts.ini tg_rss_servers -m setup --vault-password-file .vault_password | head -20

inventory:
	@echo "📋 Inventory listing..."
	@ansible-inventory -i inventory/hosts.ini --list --vault-password-file .vault_password

# CI/CD helpers
ci-setup: setup vault-init
ci-check: check
ci-deploy: deploy
ci-clean: clean

# Quick commands for common tasks
status:
	@echo "📊 System status:"
	@ansible -i inventory/hosts.ini tg_rss_servers -m shell -a "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'" --vault-password-file .vault_password 2>/dev/null || echo "Cannot get status"

restart-services:
	@echo "🔄 Restarting services..."
	@ansible-playbook -i inventory/hosts.ini playbooks/server-setup.yml --tags "restart" --vault-password-file .vault_password

update-secrets:
	@echo "🔐 Updating secrets..."
	$(MAKE) vault-edit