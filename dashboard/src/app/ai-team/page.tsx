'use client'

import { useState, useRef, useEffect } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { 
  Send, 
  Loader2, 
  ChevronDown, 
  ChevronRight,
  MessageSquare,
  Trash2,
  ArrowLeft,
  Users
} from 'lucide-react'
import { cn } from '@/lib/utils'
import Image from 'next/image'

// Agent type definition
interface Agent {
  id: string
  name: string
  title: string
  avatar: string
  department: string
  description: string
  systemPrompt: string
  color: string
}

// Department definition
interface Department {
  id: string
  name: string
  icon: string
  color: string
  description: string
}

// Message type
interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: Date
}

// Chat session with specific agent
interface AgentChatSession {
  id: string
  agentId: string
  messages: Message[]
  createdAt: Date
  updatedAt: Date
}

// Define departments
const departments: Department[] = [
  {
    id: 'engineering',
    name: 'Engineering',
    icon: '⚙️',
    color: 'from-blue-500 to-cyan-500',
    description: 'Technical implementation & infrastructure'
  },
  {
    id: 'product',
    name: 'Product',
    icon: '🎯',
    color: 'from-purple-500 to-pink-500',
    description: 'Product strategy & roadmap'
  },
  {
    id: 'support',
    name: 'Support',
    icon: '💬',
    color: 'from-green-500 to-emerald-500',
    description: 'Customer success & troubleshooting'
  },
  {
    id: 'sales',
    name: 'Sales & Growth',
    icon: '📈',
    color: 'from-orange-500 to-amber-500',
    description: 'Business development & partnerships'
  },
  {
    id: 'security',
    name: 'Security',
    icon: '🛡️',
    color: 'from-red-500 to-rose-500',
    description: 'Security, compliance & privacy'
  },
  {
    id: 'operations',
    name: 'Operations',
    icon: '🔧',
    color: 'from-slate-500 to-zinc-500',
    description: 'DevOps & infrastructure management'
  }
]

// Define agents with AI personas
const agents: Agent[] = [
  // Engineering
  {
    id: 'alex-chen',
    name: 'Alex Chen',
    title: 'Lead SDK Engineer',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=alex-chen&backgroundColor=b6e3f4',
    department: 'engineering',
    description: 'Expert in SDK development, Android/iOS integration, and API design',
    systemPrompt: 'You are Alex Chen, Lead SDK Engineer at IPLoop. You specialize in mobile SDK development, API design, and integration patterns. You communicate technically but clearly, and always provide code examples when helpful.',
    color: 'bg-blue-500'
  },
  {
    id: 'maya-patel',
    name: 'Maya Patel',
    title: 'Backend Architect',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=maya-patel&backgroundColor=c0aede',
    department: 'engineering',
    description: 'Specializes in distributed systems, proxy protocols, and performance optimization',
    systemPrompt: 'You are Maya Patel, Backend Architect at IPLoop. You excel in distributed systems design, proxy protocols (HTTP/SOCKS5), and system performance. You think architecturally and consider scalability in all solutions.',
    color: 'bg-indigo-500'
  },
  {
    id: 'james-wilson',
    name: 'James Wilson',
    title: 'DevOps Lead',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=james-wilson&backgroundColor=d1d4f9',
    department: 'engineering',
    description: 'Infrastructure, CI/CD, Kubernetes, and deployment automation expert',
    systemPrompt: 'You are James Wilson, DevOps Lead at IPLoop. You manage infrastructure, CI/CD pipelines, and Kubernetes deployments. You prioritize reliability, automation, and monitoring.',
    color: 'bg-cyan-500'
  },

  // Product
  {
    id: 'sarah-kim',
    name: 'Sarah Kim',
    title: 'Head of Product',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=sarah-kim&backgroundColor=ffd5dc',
    department: 'product',
    description: 'Product vision, roadmap planning, and feature prioritization',
    systemPrompt: 'You are Sarah Kim, Head of Product at IPLoop. You drive product strategy, gather user feedback, and prioritize features. You think in terms of user value and business impact.',
    color: 'bg-purple-500'
  },
  {
    id: 'david-lee',
    name: 'David Lee',
    title: 'UX Designer',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=david-lee&backgroundColor=ffdfbf',
    department: 'product',
    description: 'User experience design, dashboard interfaces, and usability',
    systemPrompt: 'You are David Lee, UX Designer at IPLoop. You focus on creating intuitive user experiences for the dashboard and SDK integration flows. You advocate for user-centered design.',
    color: 'bg-pink-500'
  },

  // Support
  {
    id: 'emma-garcia',
    name: 'Emma Garcia',
    title: 'Support Team Lead',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=emma-garcia&backgroundColor=c7f9cc',
    department: 'support',
    description: 'Customer onboarding, issue resolution, and documentation',
    systemPrompt: 'You are Emma Garcia, Support Team Lead at IPLoop. You help customers with onboarding, troubleshooting, and best practices. You are patient, thorough, and always provide step-by-step guidance.',
    color: 'bg-green-500'
  },
  {
    id: 'kevin-nguyen',
    name: 'Kevin Nguyen',
    title: 'Technical Support Specialist',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=kevin-nguyen&backgroundColor=bde0fe',
    department: 'support',
    description: 'SDK integration support, debugging, and technical troubleshooting',
    systemPrompt: 'You are Kevin Nguyen, Technical Support Specialist at IPLoop. You help developers integrate the SDK, debug issues, and optimize their implementations. You love diving into logs and finding root causes.',
    color: 'bg-emerald-500'
  },

  // Sales & Growth
  {
    id: 'rachel-turner',
    name: 'Rachel Turner',
    title: 'VP of Sales',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=rachel-turner&backgroundColor=fed7aa',
    department: 'sales',
    description: 'Enterprise deals, partnerships, and pricing discussions',
    systemPrompt: 'You are Rachel Turner, VP of Sales at IPLoop. You handle enterprise relationships, pricing discussions, and partnership opportunities. You understand the proxy market and competitive landscape.',
    color: 'bg-orange-500'
  },
  {
    id: 'marcus-johnson',
    name: 'Marcus Johnson',
    title: 'Partnership Manager',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=marcus-johnson&backgroundColor=fde68a',
    department: 'sales',
    description: 'SDK partnerships, integration deals, and revenue share programs',
    systemPrompt: 'You are Marcus Johnson, Partnership Manager at IPLoop. You work with app developers to integrate the SDK, discuss revenue share models, and build long-term partnerships.',
    color: 'bg-amber-500'
  },

  // Security
  {
    id: 'lisa-thompson',
    name: 'Lisa Thompson',
    title: 'Chief Security Officer',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=lisa-thompson&backgroundColor=fecaca',
    department: 'security',
    description: 'Security architecture, compliance, and privacy policies',
    systemPrompt: 'You are Lisa Thompson, Chief Security Officer at IPLoop. You oversee security architecture, compliance (GDPR, SOC2), and privacy policies. You take security seriously but explain it clearly.',
    color: 'bg-red-500'
  },
  {
    id: 'omar-hassan',
    name: 'Omar Hassan',
    title: 'Security Engineer',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=omar-hassan&backgroundColor=f9a8d4',
    department: 'security',
    description: 'Penetration testing, vulnerability management, and secure coding',
    systemPrompt: 'You are Omar Hassan, Security Engineer at IPLoop. You handle penetration testing, vulnerability assessments, and secure coding practices. You help developers write secure code.',
    color: 'bg-rose-500'
  },

  // Operations
  {
    id: 'tom-baker',
    name: 'Tom Baker',
    title: 'Head of Operations',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=tom-baker&backgroundColor=cbd5e1',
    department: 'operations',
    description: 'Node network operations, uptime monitoring, and SLAs',
    systemPrompt: 'You are Tom Baker, Head of Operations at IPLoop. You manage the global node network, monitor uptime, and ensure SLA compliance. You think in terms of reliability metrics and operational excellence.',
    color: 'bg-slate-500'
  },
  {
    id: 'anna-kowalski',
    name: 'Anna Kowalski',
    title: 'Network Engineer',
    avatar: 'https://api.dicebear.com/7.x/personas/svg?seed=anna-kowalski&backgroundColor=e2e8f0',
    department: 'operations',
    description: 'Network topology, routing optimization, and latency management',
    systemPrompt: 'You are Anna Kowalski, Network Engineer at IPLoop. You optimize network routing, manage geographic distribution of nodes, and minimize latency. You love networking protocols.',
    color: 'bg-zinc-500'
  }
]

export default function AITeamPage() {
  const [expandedDepts, setExpandedDepts] = useState<string[]>(departments.map(d => d.id))
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null)
  const [sessions, setSessions] = useState<AgentChatSession[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [showAgentList, setShowAgentList] = useState(true)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Get current session for selected agent
  const currentSession = selectedAgent 
    ? sessions.find(s => s.agentId === selectedAgent.id)
    : null
  const messages = currentSession?.messages || []

  // Load sessions from localStorage
  useEffect(() => {
    const saved = localStorage.getItem('ai-team-sessions')
    if (saved) {
      try {
        const parsed = JSON.parse(saved)
        setSessions(parsed.map((s: any) => ({
          ...s,
          createdAt: new Date(s.createdAt),
          updatedAt: new Date(s.updatedAt),
          messages: s.messages.map((m: any) => ({
            ...m,
            timestamp: new Date(m.timestamp)
          }))
        })))
      } catch (e) {
        console.error('Failed to parse saved sessions')
      }
    }
  }, [])

  // Save sessions to localStorage
  useEffect(() => {
    if (sessions.length > 0) {
      localStorage.setItem('ai-team-sessions', JSON.stringify(sessions))
    }
  }, [sessions])

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // Focus input when agent is selected
  useEffect(() => {
    if (selectedAgent) {
      inputRef.current?.focus()
    }
  }, [selectedAgent])

  const toggleDepartment = (deptId: string) => {
    setExpandedDepts(prev => 
      prev.includes(deptId) 
        ? prev.filter(id => id !== deptId)
        : [...prev, deptId]
    )
  }

  const selectAgent = (agent: Agent) => {
    setSelectedAgent(agent)
    setShowAgentList(false)
    
    // Create session if doesn't exist
    if (!sessions.find(s => s.agentId === agent.id)) {
      const newSession: AgentChatSession = {
        id: crypto.randomUUID(),
        agentId: agent.id,
        messages: [],
        createdAt: new Date(),
        updatedAt: new Date()
      }
      setSessions(prev => [...prev, newSession])
    }
  }

  const clearAgentChat = (agentId: string) => {
    setSessions(prev => prev.filter(s => s.agentId !== agentId))
  }

  const getAgentsByDepartment = (deptId: string) => {
    return agents.filter(a => a.department === deptId)
  }

  const getSessionForAgent = (agentId: string) => {
    return sessions.find(s => s.agentId === agentId)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || isLoading || !selectedAgent) return

    const userMessage: Message = {
      id: crypto.randomUUID(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date()
    }

    // Update session with user message
    setSessions(prev => prev.map(s => 
      s.agentId === selectedAgent.id 
        ? { 
            ...s, 
            messages: [...s.messages, userMessage],
            updatedAt: new Date()
          }
        : s
    ))

    setInput('')
    setIsLoading(true)

    try {
      const token = localStorage.getItem('token')
      const response = await fetch('/api/ai/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          message: userMessage.content,
          agentId: selectedAgent.id,
          agentName: selectedAgent.name,
          systemPrompt: selectedAgent.systemPrompt,
          history: messages.slice(-10) // Send last 10 messages for context
        })
      })

      if (!response.ok) {
        throw new Error('Failed to get response')
      }

      const data = await response.json()
      
      const assistantMessage: Message = {
        id: crypto.randomUUID(),
        role: 'assistant',
        content: data.response || `I apologize, I'm having trouble responding right now. Please try again.`,
        timestamp: new Date()
      }

      setSessions(prev => prev.map(s => 
        s.agentId === selectedAgent.id 
          ? { 
              ...s, 
              messages: [...s.messages, assistantMessage],
              updatedAt: new Date()
            }
          : s
      ))
    } catch (error) {
      console.error('Chat error:', error)
      const errorMessage: Message = {
        id: crypto.randomUUID(),
        role: 'assistant',
        content: 'I apologize, but I encountered an error. Please try again in a moment.',
        timestamp: new Date()
      }
      setSessions(prev => prev.map(s => 
        s.agentId === selectedAgent.id 
          ? { ...s, messages: [...s.messages, errorMessage] }
          : s
      ))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Layout>
      <div className="flex h-[calc(100vh-8rem)] gap-4">
        {/* Agent List Sidebar */}
        <Card className={cn(
          "w-80 flex-shrink-0 flex flex-col transition-all duration-300",
          !showAgentList && "hidden lg:flex"
        )}>
          <CardHeader className="pb-2 border-b">
            <div className="flex items-center gap-2">
              <Users className="h-5 w-5 text-primary" />
              <CardTitle className="text-lg">AI Team</CardTitle>
            </div>
            <CardDescription>
              Chat with specialized AI agents
            </CardDescription>
          </CardHeader>
          <CardContent className="flex-1 overflow-hidden p-0">
            <ScrollArea className="h-full">
              <div className="p-2 space-y-1">
                {departments.map(dept => (
                  <div key={dept.id}>
                    {/* Department Header */}
                    <button
                      onClick={() => toggleDepartment(dept.id)}
                      className="w-full flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-accent transition-colors"
                    >
                      {expandedDepts.includes(dept.id) ? (
                        <ChevronDown className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-muted-foreground" />
                      )}
                      <span className="text-lg">{dept.icon}</span>
                      <span className="font-medium text-sm">{dept.name}</span>
                      <Badge variant="secondary" className="ml-auto text-xs">
                        {getAgentsByDepartment(dept.id).length}
                      </Badge>
                    </button>

                    {/* Agents in Department */}
                    {expandedDepts.includes(dept.id) && (
                      <div className="ml-6 space-y-1 mt-1">
                        {getAgentsByDepartment(dept.id).map(agent => {
                          const hasMessages = getSessionForAgent(agent.id)?.messages.length ?? 0 > 0
                          return (
                            <button
                              key={agent.id}
                              onClick={() => selectAgent(agent)}
                              className={cn(
                                "w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors text-left group",
                                selectedAgent?.id === agent.id
                                  ? "bg-primary text-primary-foreground"
                                  : "hover:bg-accent"
                              )}
                            >
                              <div className="relative">
                                <img
                                  src={agent.avatar}
                                  alt={agent.name}
                                  className="h-10 w-10 rounded-full bg-muted"
                                />
                                {hasMessages && (
                                  <div className="absolute -top-1 -right-1 h-3 w-3 bg-green-500 rounded-full border-2 border-background" />
                                )}
                              </div>
                              <div className="flex-1 min-w-0">
                                <p className="font-medium text-sm truncate">{agent.name}</p>
                                <p className={cn(
                                  "text-xs truncate",
                                  selectedAgent?.id === agent.id
                                    ? "text-primary-foreground/70"
                                    : "text-muted-foreground"
                                )}>
                                  {agent.title}
                                </p>
                              </div>
                              {hasMessages && (
                                <MessageSquare className={cn(
                                  "h-4 w-4 opacity-50",
                                  selectedAgent?.id === agent.id
                                    ? "text-primary-foreground"
                                    : "text-muted-foreground"
                                )} />
                              )}
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        {/* Chat Area */}
        <Card className="flex-1 flex flex-col">
          {selectedAgent ? (
            <>
              {/* Chat Header */}
              <CardHeader className="border-b py-3">
                <div className="flex items-center gap-3">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="lg:hidden"
                    onClick={() => setShowAgentList(true)}
                  >
                    <ArrowLeft className="h-5 w-5" />
                  </Button>
                  <img
                    src={selectedAgent.avatar}
                    alt={selectedAgent.name}
                    className="h-12 w-12 rounded-full bg-muted"
                  />
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <CardTitle className="text-lg">{selectedAgent.name}</CardTitle>
                      <Badge 
                        variant="secondary" 
                        className={cn("text-xs", selectedAgent.color, "text-white")}
                      >
                        {departments.find(d => d.id === selectedAgent.department)?.name}
                      </Badge>
                    </div>
                    <CardDescription>{selectedAgent.title}</CardDescription>
                  </div>
                  {messages.length > 0 && (
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => clearAgentChat(selectedAgent.id)}
                      title="Clear chat history"
                    >
                      <Trash2 className="h-4 w-4 text-muted-foreground" />
                    </Button>
                  )}
                </div>
              </CardHeader>

              {/* Messages */}
              <CardContent className="flex-1 overflow-hidden p-0">
                <ScrollArea className="h-full p-4">
                  {messages.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-full text-center px-4">
                      <img
                        src={selectedAgent.avatar}
                        alt={selectedAgent.name}
                        className="h-24 w-24 rounded-full bg-muted mb-4"
                      />
                      <h3 className="text-lg font-semibold mb-2">
                        Chat with {selectedAgent.name}
                      </h3>
                      <p className="text-muted-foreground mb-4 max-w-md">
                        {selectedAgent.description}
                      </p>
                      <div className="flex flex-wrap gap-2 justify-center max-w-md">
                        {getSuggestedQuestions(selectedAgent).map((q, i) => (
                          <Button
                            key={i}
                            variant="outline"
                            size="sm"
                            className="text-xs"
                            onClick={() => {
                              setInput(q)
                              inputRef.current?.focus()
                            }}
                          >
                            {q}
                          </Button>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-4">
                      {messages.map(message => (
                        <div
                          key={message.id}
                          className={cn(
                            "flex gap-3",
                            message.role === 'user' ? "justify-end" : "justify-start"
                          )}
                        >
                          {message.role === 'assistant' && (
                            <img
                              src={selectedAgent.avatar}
                              alt={selectedAgent.name}
                              className="h-8 w-8 rounded-full bg-muted flex-shrink-0"
                            />
                          )}
                          <div
                            className={cn(
                              "rounded-lg px-4 py-2 max-w-[80%]",
                              message.role === 'user'
                                ? "bg-primary text-primary-foreground"
                                : "bg-muted"
                            )}
                          >
                            <p className="text-sm whitespace-pre-wrap">{message.content}</p>
                            <p className={cn(
                              "text-xs mt-1",
                              message.role === 'user' 
                                ? "text-primary-foreground/70" 
                                : "text-muted-foreground"
                            )}>
                              {message.timestamp.toLocaleTimeString([], { 
                                hour: '2-digit', 
                                minute: '2-digit' 
                              })}
                            </p>
                          </div>
                          {message.role === 'user' && (
                            <div className="h-8 w-8 rounded-full bg-secondary flex items-center justify-center flex-shrink-0">
                              <span className="text-sm font-medium">You</span>
                            </div>
                          )}
                        </div>
                      ))}
                      {isLoading && (
                        <div className="flex gap-3 justify-start">
                          <img
                            src={selectedAgent.avatar}
                            alt={selectedAgent.name}
                            className="h-8 w-8 rounded-full bg-muted flex-shrink-0"
                          />
                          <div className="bg-muted rounded-lg px-4 py-3">
                            <div className="flex gap-1">
                              <div className="w-2 h-2 bg-muted-foreground/50 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                              <div className="w-2 h-2 bg-muted-foreground/50 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                              <div className="w-2 h-2 bg-muted-foreground/50 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                            </div>
                          </div>
                        </div>
                      )}
                      <div ref={messagesEndRef} />
                    </div>
                  )}
                </ScrollArea>
              </CardContent>

              {/* Input */}
              <div className="border-t p-4">
                <form onSubmit={handleSubmit} className="flex gap-2">
                  <Input
                    ref={inputRef}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder={`Message ${selectedAgent.name}...`}
                    disabled={isLoading}
                    className="flex-1"
                  />
                  <Button type="submit" disabled={isLoading || !input.trim()}>
                    {isLoading ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Send className="h-4 w-4" />
                    )}
                  </Button>
                </form>
              </div>
            </>
          ) : (
            /* No Agent Selected State */
            <div className="flex-1 flex flex-col items-center justify-center text-center px-8">
              <div className="h-20 w-20 rounded-full bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center mb-6">
                <Users className="h-10 w-10 text-primary" />
              </div>
              <h2 className="text-2xl font-bold mb-2">Welcome to AI Team</h2>
              <p className="text-muted-foreground mb-6 max-w-md">
                Select an agent from the sidebar to start a focused conversation. 
                Each agent specializes in different areas of IPLoop.
              </p>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-4 max-w-2xl">
                {departments.slice(0, 6).map(dept => (
                  <Card 
                    key={dept.id} 
                    className="p-4 cursor-pointer hover:border-primary transition-colors"
                    onClick={() => {
                      const deptAgents = getAgentsByDepartment(dept.id)
                      if (deptAgents.length > 0) {
                        selectAgent(deptAgents[0])
                      }
                    }}
                  >
                    <div className={cn(
                      "h-10 w-10 rounded-lg bg-gradient-to-br flex items-center justify-center text-xl mb-2",
                      dept.color
                    )}>
                      {dept.icon}
                    </div>
                    <h3 className="font-medium text-sm">{dept.name}</h3>
                    <p className="text-xs text-muted-foreground">
                      {getAgentsByDepartment(dept.id).length} agents
                    </p>
                  </Card>
                ))}
              </div>
            </div>
          )}
        </Card>
      </div>
    </Layout>
  )
}

// Helper function for suggested questions per agent
function getSuggestedQuestions(agent: Agent): string[] {
  const questions: Record<string, string[]> = {
    'alex-chen': [
      'How do I integrate the SDK?',
      'Show me a code example',
      'What are the SDK requirements?'
    ],
    'maya-patel': [
      'Explain the proxy architecture',
      'How does load balancing work?',
      'What protocols are supported?'
    ],
    'james-wilson': [
      'How do I deploy to production?',
      'What monitoring is available?',
      'Explain the CI/CD pipeline'
    ],
    'sarah-kim': [
      'What features are on the roadmap?',
      'How do you prioritize features?',
      'Tell me about the product vision'
    ],
    'david-lee': [
      'How can I customize the UI?',
      'What are UX best practices?',
      'Help with dashboard design'
    ],
    'emma-garcia': [
      'Help me get started',
      'Troubleshoot my connection',
      'Where is the documentation?'
    ],
    'kevin-nguyen': [
      'Debug my SDK integration',
      'Why is my connection slow?',
      'Check my configuration'
    ],
    'rachel-turner': [
      'What are the pricing tiers?',
      'Enterprise options available?',
      'Volume discounts?'
    ],
    'marcus-johnson': [
      'SDK partnership program?',
      'Revenue share details?',
      'Integration requirements?'
    ],
    'lisa-thompson': [
      'Is IPLoop GDPR compliant?',
      'Security certifications?',
      'Data privacy policies?'
    ],
    'omar-hassan': [
      'Security best practices?',
      'How is traffic encrypted?',
      'Vulnerability reporting?'
    ],
    'tom-baker': [
      'Current network status?',
      'What is the uptime SLA?',
      'How many nodes are active?'
    ],
    'anna-kowalski': [
      'Optimize my latency',
      'Geographic coverage?',
      'Routing strategies?'
    ]
  }
  return questions[agent.id] || ['How can you help me?', 'Tell me about your role', 'What do you specialize in?']
}
