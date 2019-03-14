require 'open-uri'
require 'yaml'


module NCP
  #def self.download source, target
  #  File.open(target, 'wb') {|f| f.write(open(source) {|f1| f1.read})}
  #end

  def self.download file, source
    config = $ncp[__method__.to_s]
    #config = YAML.load_file('./config.yml')['ncp']
    #pp $ncp
    $log.info "Ncp == #{__method__} #{file} #{source}"
    File.open(config[file], 'wb') {|f| f.write(open(source) {|f1| f1.read})}
  end

  def self.status
    config = $ncp[__method__.to_s]
  end
end
