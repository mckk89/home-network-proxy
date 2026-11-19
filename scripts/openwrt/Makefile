#
# 家庭网络反代 OpenWrt Package Makefile
# Copyright (C) 2024 gdy666
#
# This is free software, licensed under the GNU General Public License v3.
#

include $(TOPDIR)/rules.mk

PKG_NAME:=hnproxy
PKG_VERSION:=3.5.0
PKG_RELEASE:=1

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://github.com/mckk89/home-network-proxy/releases/download/v$(PKG_VERSION)/
PKG_HASH:=skip

PKG_LICENSE:=MIT
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=gdy666 <gdy666@66666.host>

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_USE_MIPS16:=0

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/hnproxy
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=家庭网络反代 - Personal and Home Network Tool
  URL:=https://lucky666.cn/
  DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +kmod-tun +ipset +iptables-mod-nat-extra
  PROVIDES:=hnproxy
  VARIANT:=full
endef

define Package/hnproxy/description
 家庭网络反代 is a networking tool for personal and home networks.
 Its web-based management interface helps you manage dynamic DNS,
 access to local services, certificates, file services, and device automation.
 
 Features: DDNS, Reverse Proxy, Port Forwarding, SSL/ACME, 
 WebDAV, FTP, Docker Management, Web Terminal, WAF, etc.
endef

# 定义不同变体
define Package/hnproxy-variant
  $(call Package/hnproxy)
  VARIANT:=$(1)
  TITLE:=$(2)
  DEPENDS:=$(3)
endef

$(eval $(call Package/hnproxy-variant,lite,轻量版 - 最精简,$(GO_ARCH_DEPENDS) +ca-bundle))
$(eval $(call Package/hnproxy-variant,standard,标准版 - 基础功能,$(GO_ARCH_DEPENDS) +ca-bundle))
$(eval $(call Package/hnproxy-variant,medium,增强版 - 中等功能,$(GO_ARCH_DEPENDS) +ca-bundle))
$(eval $(call Package/hnproxy-variant,full,完整版 - 完整功能,$(GO_ARCH_DEPENDS) +ca-bundle))
$(eval $(call Package/hnproxy-variant,lite_waf,轻量 WAF 版 - 含 WAF,$(GO_ARCH_DEPENDS) +ca-bundle))

# 编译标签映射
VARIANT_TAGS_lite:=lite
VARIANT_TAGS_standard:=standard
VARIANT_TAGS_medium:=medium
VARIANT_TAGS_full:=full
VARIANT_TAGS_lite_waf:=lite_waf

GO_LDFLAGS:=-s -w -X main.Version=$(PKG_VERSION) -X main.BuildTime=$$(date -u '+%Y-%m-%d_%H:%M:%S')

define Build/Compile
	$(call GoPackage/Build/Compile)
endef

define Package/hnproxy/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/hnproxy $(1)/usr/bin/hnproxy
	
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/hnproxy.init $(1)/etc/init.d/hnproxy
	
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/hnproxy.config $(1)/etc/config/hnproxy
	
	$(INSTALL_DIR) $(1)/etc/hnproxy
	$(INSTALL_CONF) ./files/hnproxy.conf.example $(1)/etc/hnproxy/hnproxy.conf
endef

define Package/hnproxy/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] || {
	/etc/init.d/hnproxy enable
	/etc/init.d/hnproxy start
}
exit 0
endef

define Package/hnproxy/prerm
#!/bin/sh
/etc/init.d/hnproxy stop
/etc/init.d/hnproxy disable
exit 0
endef

$(eval $(call BuildPackage,hnproxy))
$(eval $(call BuildPackage,hnproxy-lite))
$(eval $(call BuildPackage,hnproxy-standard))
$(eval $(call BuildPackage,hnproxy-medium))
$(eval $(call BuildPackage,hnproxy-full))
$(eval $(call BuildPackage,hnproxy-lite_waf))