require "digest"
require "tmpdir"

class Rail < Formula
  source_repo = if ENV["HOMEBREW_RAIL_SOURCE_REPO"].to_s.empty?
    (Pathname.new(__dir__) / "../..").realpath
  else
    Pathname.new(ENV["HOMEBREW_RAIL_SOURCE_REPO"]).realpath
  end
  source_archive_dir = Pathname.new(Dir.mktmpdir("rail-formula-source"))
  source_archive = source_archive_dir/"rail-0.0.0.tar.gz"

  system "tar", "-czf", source_archive.to_s, "-C", source_repo.to_s, "."

  desc "Harness control-plane for Codex"
  homepage "https://example.com/rail"
  url "file://#{source_archive}"
  version "0.0.0"
  sha256 Digest::SHA256.file(source_archive).hexdigest
  license "MIT"

  SOURCE_REPO = source_repo
  CODEX_HOME = if ENV["HOMEBREW_RAIL_CODEX_HOME"].to_s.empty?
    File.join(Dir.home, ".codex")
  else
    ENV["HOMEBREW_RAIL_CODEX_HOME"]
  end

  depends_on "go" => :build

  def install
    source_dir = buildpath/"source-tree"
    source_dir.mkpath

    SOURCE_REPO.children.each do |path|
      next if [".git", ".dart_tool", ".worktrees", "build"].include?(path.basename.to_s)

      cp_r path, source_dir
    end

    cd source_dir do
      system "go", "build", "-trimpath", "-o", bin/"rail", "./cmd/rail"
      pkgshare.install "assets/skill"
    end
  end

  def post_install
    target = Pathname.new(CODEX_HOME)/"skills"/"rail"
    source = pkgshare/"skill"/"Rail"

    rm_rf target
    target.parent.mkpath
    target.mkpath
    cp_r source.children, target
  end

  def caveats
    <<~EOS
      Rail installs its packaged Codex skill assets under:
        #{opt_pkgshare}/skill/Rail

      A Codex-discoverable copy is also materialized under:
        #{CODEX_HOME}/skills/rail
    EOS
  end

  test do
    assert_match "compose-request", shell_output("#{bin}/rail compose-request 2>&1", 1)
    assert_predicate pkgshare/"skill"/"Rail"/"SKILL.md", :exist?
  end
end
