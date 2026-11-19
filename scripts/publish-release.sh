# 家庭网络反代 v3.5.0 发布脚本
# 用法: ./scripts/publish-release.sh [版本号]

set -euo pipefail

VERSION="${1:-3.5.0}"
GITHUB_REPO="${GITHUB_REPO:-<你的用户名>/家庭反代}"
DIST_DIR="dist/${VERSION}"
RELEASE_TAG="v${VERSION}"

# 1. 构建全部平台产物
echo "=== 构建 ${VERSION} ==="
./scripts/build.sh "${VERSION}"

# 2. 收集要上传的包（tar.gz + zip + 校验和）
echo "=== 收集发布包 ==="
UPLOAD_FILES=()
while IFS= read -r f; do
    UPLOAD_FILES+=("$DIST_DIR/$f")
done < <(cd "$DIST_DIR" && ls *.tar.gz *.zip SHA256SUMS 2>/dev/null)

echo "共 ${#UPLOAD_FILES[@]} 个文件："
for f in "${UPLOAD_FILES[@]}"; do echo "  - $f"; done

# 3. 创建 GitHub Release 并上传
echo "=== 创建 GitHub Release ==="
gh release create "${RELEASE_TAG}" \
    --title "家庭网络反代 ${VERSION}" \
    --notes-file docs/release-notes-v3.5.0.md \
    --target main \
    --repo "${GITHUB_REPO}" \
    "${UPLOAD_FILES[@]}"

echo "=== 完成 ==="
echo "Release 地址: https://github.com/${GITHUB_REPO}/releases/${RELEASE_TAG}"
echo "下载页:      https://github.com/${GITHUB_REPO}/releases"
