require "sinatra"

get "/*.*" do |path, ext|
  if ext == "html" || ext == "htm" || ext == ""
    redirect "/404.html"
  end
end