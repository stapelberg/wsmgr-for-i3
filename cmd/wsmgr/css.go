package main

const css = `
#win,
#grid,
#icon,
#title,
#message,
#btnunlock {
  font-weight: bold;
  font-style: italic;
  color: #ffffff;
  background: #000000;
}

#grid {
  margin: 1rem;
  border: 1px solid grey;
  padding-left: 1rem;
  padding-right: 1rem;
}

#icon {
  margin-top: 1rem;
  /*background-color: yellow;*/
}

#title {
  padding: 1rem;
  font-size: 150%;
}

#message {
  padding: 1rem;
}

#btnunlock {
  margin: 1rem;
  padding: 1rem;
  border: 1px solid green;
}

#btnunlock:hover {
  background-color: #555;
}

#btnunlock:active {
  background-color: blue;
}
`
